package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

var debug_flag *bool

type IP [4]uint8

func (ip IP) String() string {
	var output string = ""
	for index, octet := range ip {
		output += fmt.Sprint(octet)
		if index != 3 {
			output += "."
		}
	}
	return output
}

type MAC [6]uint8

var broadcast_mac MAC = MAC{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

func (mac MAC) String() string {
	return fmt.Sprintf("%x.%x.%x.%x.%x.%x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

type Opcode uint16
const (
	ARP_REQUEST Opcode = iota
	ARP_REPLY   Opcode = iota
)

type FramePayload struct {
	Data interface{}
}

type ARPPayload struct {
     HardwareType uint16
     ProtocolType uint16
     HardwareAddrLen uint8
     ProtocolAddrLen uint8
     OperationType Opcode
     SourceHardwareAddr MAC // TODO this should be uint48
     SourceProtocolAddr IP // TODO this should be uint32
     DestHardwareAddr MAC
     DestProtocolAddr IP
}

type EtherType uint16
const (
      IPv4 EtherType = 0x0800
      ARP EtherType = 0x0806
)

type Frame struct {
	destMac   MAC
	sourceMac MAC
	etherType EtherType
	payload   FramePayload // TODO Need to find a way to make this a variably sized polymorphic struct
		  	       // It's also possible to just make it a byte field but that wouldn't be very abstract
}
func (frame Frame) String() string {
     str:= fmt.Sprintf("Dest:%v, Source:%v, EtherType:%v,", frame.destMac, frame.sourceMac, frame.etherType)
     switch frame.etherType {
     case ARP:
     	  if payload, correct_type := frame.payload.Data.(ARPPayload); !correct_type {
	     	str += "Error:Incorrect payload type for etherType"
	  } else {
		str += fmt.Sprintf("ARPPayload{OperationType: %v}", payload.OperationType)
	  }
     default:
	str += "Error:Unknown ethertype"
     }
     return str

}

type Interface struct {
	Name string
	Ip   IP
	Mac  MAC
	// Denotes where this interface is connected (nil if no connection)
	Connection chan Frame // channels represent an ethernet connection
	Connected_to chan Frame // TODO both these channels need to be buffers to prevent deadlock
}

func (intf *Interface) Send(frame Frame) {
     if intf != nil && intf.Connected_to != nil {
	intf.Connected_to <- frame
     }
}

func (intf Interface) String() string {
	var output string = ""
	output += "Name:" + intf.Name
	output += ","
	output += "IP:" + intf.Ip.String()
	output += ","
	output += "MAC:" + intf.Mac.String()
	return output
}

// A session allows a procedure to set a watch on incoming frames to see if a matching frame comes in. The frame will get sent over the channel to notify the procedure that its expected frame has arrived
type Session struct {
     IsMatchingFrame func(Frame) bool
     Response chan Frame
}

type Host struct {
	// TODO Need list of interfaces
	// TODO add hostname
	Intf      Interface
	arp_table map[IP]MAC
	Sessions []Session
	SessionsMutex sync.Mutex
}

func (host *Host) String() string {
	return host.Intf.String() + "," + fmt.Sprint(host.arp_table)
}

func NewHost(ip IP, mac MAC) *Host {
	return &Host{Interface{"eth0" /*TODO*/, ip, mac, make(chan Frame), nil}, make(map[IP]MAC), nil, sync.Mutex{}}
}

type Switch struct {
	Name      string
	Ports     []Interface
	Mac_table map[MAC]int // TODO access to this map may need to be synchronized (if a separate thread will be initiating sends on the network)
}

func NewSwitch(ports int) *Switch {
	var swt Switch
	swt.Name = "sw0"
	swt.Ports = make([]Interface, ports)
	for i := range swt.Ports {
	    ip := IP{0,0,0,0} /*TODO*/
	    mac := MAC{uint8(rand.Intn(256)),uint8(rand.Intn(256)),uint8(rand.Intn(256)),uint8(rand.Intn(256)),uint8(rand.Intn(256)),uint8(rand.Intn(256))}
	    
	    swt.Ports[i] = Interface{"eth0" /*TODO*/, ip, mac, make(chan Frame), nil}
	}
	swt.Mac_table = make(map[MAC]int)
	return &swt
}

func (swt *Switch) PowerOn() {
     var cases []reflect.SelectCase = make([]reflect.SelectCase,len(swt.Ports))
     for i := range swt.Ports {
     	 cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(swt.Ports[i].Connection)}
     }
     
     for {
          // reflect select
     	  port_num, recv_frame, _ := reflect.Select(cases)
	  swt.ReceiveFrame(recv_frame.Interface().(Frame), port_num)
     }
}

func (swt *Switch) ReceiveFrame(frame Frame, recv_port_num int) {
	if swt != nil {
		if _, found := swt.Mac_table[frame.sourceMac]; !found {
			swt.Mac_table[frame.sourceMac] = recv_port_num
		}
		swt.SendFrame(frame, recv_port_num)
	}
}

// TODO Should not send frame out of the same port it was received on
func (swt *Switch) SendFrame(frame Frame, recv_port_num int) {
     if port_num, found := swt.Mac_table[frame.destMac]; found {
     	swt.Ports[port_num].Send(frame)
     } else {
       for i, intf := range swt.Ports {
       	   if i != recv_port_num {
       	      intf.Send(frame)
	   }
       }
     }
}

// This is a meta function that allows the user to simulate
// connecting two hosts on the network by ethernet
// TODO any synchronization required here? Yes, because inptf_1 and intf_2 need to be simultaneuously connected
// Otherwise, if the connecting thread decides to stall for a while, it is possible that a message could be sent across to intf_2
// and intf_2 is still not connected back to intf_1 to allow for a response
func Connect(intf_1, intf_2 *Interface) error {
	if intf_1 == nil && intf_2 == nil {
		return errors.New("Connect(...): intf_1 and intf_2 are nil")
	} else if intf_1 == nil {
		return errors.New("Connect(...): intf_1 is nil")
	} else if intf_2 == nil {
		return errors.New("Connect(...): intf_2 is nil")
	}
	// TODO need synchronization, see todo at method signature
	intf_1.Connected_to = intf_2.Connection
	intf_2.Connected_to = intf_1.Connection
	return nil
}

func NewArpRequestFrame(sourceMac MAC, sourceIp IP, destIp IP) Frame {
     return NewArpFrame(sourceMac, sourceIp, broadcast_mac, destIp, ARP_REQUEST)
}
func NewArpReplyFrame(sourceMac MAC, sourceIp IP, destMac MAC, destIp IP) Frame {
     return NewArpFrame(sourceMac, sourceIp, destMac, destIp, ARP_REPLY)
}
func NewArpFrame(sourceMac MAC, sourceIp IP, destMac MAC, destIp IP, op Opcode) Frame {
     var frame Frame
     frame.sourceMac = sourceMac
     frame.destMac = destMac
     frame.etherType = ARP
     frame.payload.Data = ARPPayload{OperationType: op,
			SourceHardwareAddr: sourceMac,
			SourceProtocolAddr: sourceIp,
			DestHardwareAddr: destMac,
			DestProtocolAddr: destIp}
     return frame
}

/* Road Map:
1. Implement simple layer 3 (IPv4 only?) packet sending
2. Implement Routers and static routing
3. Implement ICMP (ping / traceroute)
4. Implement RIP ? Maybe?
5. Implement UDP/TCP
*/

type Packet struct {
     PacketMessage string
}

//func (host *Host) SendPacket(packet Packet) {

/* If you want to send a packet, to a particular IP address, you have to first know the MAC Address.
   If the MAC for the destination IP is not in the routing table, we need to send out an ARP request
   and wait on the reply (this implies that SendPacket must have a way to wait for the return of the ARP Reply...)
   Then, after receiving the reply, a regular frame gets sent to the destination using SendFrame

   Best approach for doing this would be adding a slice of "open sessions" to the host, and determining if incoming
   frames / packets are meant to be received by any of the open sessions. The open session would then be activated
   for further processing, probably passing the the relevant frame/packet to the open session. A channel would be a
   very natural way of representing this because it would allow for both synchronization between the open session
   resuming processing (e.g. after an ARP reply came in) as well as the transfer of the required data for that
   processing (e.g the ARP reply itself). Granted, for the current ARP application, the method for receiving Frames
   from a channel already processes those ARP replies and modifies the ARP table. At least for this application,
   specifically, the only synchronization required would be to have an ARP request wait for the completion of the
   current processing of the ARP packet. Not sure if the design could be improved by moving this processing into
   the areas where the response needs to be waited on. However, it might be really hard to implement TCP sessions?
   We will see... Maybe TCP can just be a slice of TCP sessions on the host?
*/
/*
     if _, found := host.arp_table[destIP]; !found {
       if *debug_flag {fmt.Println("SendPacket(...): Sending ARP Request...")}
	host.SendArpRequest(destIp)
       if *debug_flag {fmt.Println("SendPacket(...): Received ARP Reply...")}
     }
     if mac, found := host.arp_table[destIP]; found {
          host.SendFrame(Frame{destMac: mac, sourceMac: host.Intf.Mac, etherType:IPv4, payload:FramePayload{"This is an IPv4 Test Message"})
     } else {
       fmt.Println("SendPacket(...): The ARP Request probably timed out, no MAC address found for destination IP")
     }
     
}
*/

func (host *Host) SendArpRequest(destIp IP) {
     	var arp_reply_chan chan Frame = make(chan Frame)
     	arp_request_session := Session{
		    func(frame Frame) bool {
			if payload, correct_type := frame.payload.Data.(ARPPayload); 
			   frame.destMac == host.Intf.Mac && correct_type && 
			   payload.SourceProtocolAddr == destIp && 
			   payload.DestProtocolAddr == host.Intf.Ip && 
			   payload.DestHardwareAddr == host.Intf.Mac {
			   			    return true
			   } else {
			     return false
			   }
	            },
		    arp_reply_chan}
	host.SessionsMutex.Lock() 
     	host.Sessions = append(host.Sessions, arp_request_session) // A session is composed of a Frame comparison function and a channel to notify on if that function returns true
	host.SessionsMutex.Unlock()
     	host.SendFrame(NewArpRequestFrame(host.Intf.Mac, host.Intf.Ip, destIp))
	// Wait until the ARP request has been answered
	<- arp_reply_chan // Nothing needs to be done with the output frame from the channel because ARP replies get handled automatically in ReceiveFrame such that we are able to accomodate unsolicited ARP announcements
	if *debug_flag {fmt.Println("SendArpRequest(...): Received ARP Reply frame from Channel")}
	
}

func RemoveFrameFromSlice(slice []Session, index_to_remove int) []Session {
     slice = append(slice[:index_to_remove], slice[index_to_remove+1:]...)
     return slice
}

func (host *Host) SendFrameToMatchingSessions(frame Frame, removeSessions bool) {
     host.SessionsMutex.Lock()
     for i, session := range host.Sessions {
     	 if session.IsMatchingFrame(frame) {
	    session.Response <- frame
	    if removeSessions {
	       host.Sessions = RemoveFrameFromSlice(host.Sessions,i)
	    }
	 }
     }
     host.SessionsMutex.Unlock()
}

/*
func ProcessArpFrame(frame Frame) {
     if frame.destIp == host.Intf.Ip {
     	// Form the reply frame
	var reply Frame
	reply.destMac = frame.sourceMac
	reply.sourceMac = host.Intf.Mac
	reply.destIp = frame.sourceIp
	reply.sourceIp = host.Intf.Ip

	// Send it back out the same interface
     	host.ForwardFrame(
     }
     // For receiving an ARP reply
     if frame.type == ARP && frame.destMac == host.Intf.Mac && frame.destIp == host.Intf.Ip {

     }
}
*/

func (host *Host) PowerOn() {
     var cases []reflect.SelectCase = make([]reflect.SelectCase,1)
     cases[0] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(host.Intf.Connection)}
     
     for {
          // reflect select
     	  _, recv_frame, _ := reflect.Select(cases)
	  // This is the management port case
	  host.ReceiveFrame(recv_frame.Interface().(Frame), &(host.Intf))
     }
}

func (host *Host) ReceiveFrame(frame Frame, intf *Interface) {
	if host != nil {
		if *debug_flag {fmt.Println("ReceiveFrame(...): Host", host, "receiving frame", frame)}
		if frame.destMac == host.Intf.Mac || frame.destMac == broadcast_mac {
			switch frame.etherType {
			       case ARP: // Put each case into its own method
			       if payload, correct_type := frame.payload.Data.(ARPPayload); !correct_type {
			       	  fmt.Printf("ReceiveFrame(...): Error, FramePayload was incorrect type %t\n", frame.payload.Data)
			       } else {
			       	 if payload.DestProtocolAddr == host.Intf.Ip {
				 switch payload.OperationType {
			       	 	case ARP_REQUEST:
				 	reply := NewArpReplyFrame(host.Intf.Mac, host.Intf.Ip, payload.SourceHardwareAddr, payload.SourceProtocolAddr)
				 	host.SendFrame(reply)
			       	 	case ARP_REPLY:
				      	fmt.Println("ReceiveFrame(...): Received ARP_REPLY")
					// Add Reply to arp_table regardless of whether this arp reply was solicited or unsolicited
				      	host.arp_table[payload.SourceProtocolAddr] = payload.SourceHardwareAddr
					// Notify host that an ARP reply was received (if it is waiting for one)
					host.SendFrameToMatchingSessions(frame, true)
					
				 }
			      }
			      }
		      }
		}
	}
}

func (host *Host) SendFrame(frame Frame) {
	if host != nil {
		if *debug_flag {fmt.Println("SendFrame(...): Host", host, "sending frame", frame)}
		// Send the frame out of our interface to wherever our interface is connected to
		host.Intf.Send(frame)
	}
}

// TODO Everything more above and including an ARP request should probably be run in parallel because it needs to wait on the response after sending (Unless that logic can be worked into the ReceiveFrame method)
// TODO The network actually could be deterministic even with parallel devices, if everything is calculated in steps (with a separate sending stage most likely). e.g. Every network tick, all devices receive once (there will be a barrier at the bottom of the infinite receive for loop, and a default case for no receives), process the packets, and prepare to send. Then they all wait on a barrier to send, then they all send. Only problem is that sends would all now need to be run as go routines because otherwise there would be deadlock on every send (if a switch wants to send, it will wait on the channel until somone receives, but all the other devices will be waiting at a barrier for their next read, and will never become ready to read. This could be fixed with channel buffers that don't block when writing, but that can come later

// TODO To allow for sessions (and anything else that requires continuity between a send and subsequent receives), there should probably be a stack of "receivers" for running processes that need to act on specific received inputs that are related to packets they sent out earlier
// One good example of the above is when a host wants to send a packet but needs to do an ARP request first. It needs to wait for the ARP request to be returned before it can correctly address the packet it wants to send across the network

func main() {
     debug_flag = flag.Bool("DEBUG", false, "a bool")
     flag.Parse()
     
     	fmt.Println("Test 1 ======================================")
	host_1 := NewHost(IP{10, 0, 0, 1}, MAC{0, 0, 0, 0, 0, 1})
	go host_1.PowerOn()
	host_2 := NewHost(IP{10, 0, 0, 2}, MAC{0, 0, 0, 0, 0, 2})
	go host_2.PowerOn()
	Connect(&(host_1.Intf), &(host_2.Intf))
	host_1.SendFrame(NewArpRequestFrame(host_1.Intf.Mac, host_1.Intf.Ip, host_2.Intf.Ip))
	time.Sleep(100000000)
	fmt.Println(host_1)
	time.Sleep(100000000)

	fmt.Println("Test 2 ======================================")
	host_3 := NewHost(IP{10, 0, 0, 3}, MAC{0, 0, 0, 0, 0, 3})
	go host_3.PowerOn()
	host_4 := NewHost(IP{10, 0, 0, 4}, MAC{0, 0, 0, 0, 0, 4})
	go host_4.PowerOn()
	swt_1 := NewSwitch(4)
	go swt_1.PowerOn()
	Connect(&(host_3.Intf), &(swt_1.Ports[0]))
	Connect(&(host_4.Intf), &(swt_1.Ports[1]))
	host_3.SendFrame(NewArpRequestFrame(host_3.Intf.Mac, host_3.Intf.Ip, host_4.Intf.Ip))
	time.Sleep(100000000)

     	fmt.Println("Test 3 ======================================")
	// Should get no response
	host_3.SendFrame(NewArpRequestFrame(host_3.Intf.Mac, host_3.Intf.Ip, host_1.Intf.Ip))
	time.Sleep(100000000)

     	fmt.Println("Test 4 (Two Switches) ======================================")
	host_5 := NewHost(IP{10, 0, 0, 5}, MAC{0, 0, 0, 0, 0, 5})
	go host_5.PowerOn()
	host_6 := NewHost(IP{10, 0, 0, 6}, MAC{0, 0, 0, 0, 0, 6})
	go host_6.PowerOn()
	swt_2 := NewSwitch(2)
	go swt_2.PowerOn()
	swt_3 := NewSwitch(2)
	go swt_3.PowerOn()
	Connect(&(host_5.Intf), &(swt_2.Ports[0]))
	Connect(&(host_6.Intf), &(swt_3.Ports[0]))
	Connect(&(swt_2.Ports[1]), &(swt_3.Ports[1]))
	host_5.SendFrame(NewArpRequestFrame(host_5.Intf.Mac, host_5.Intf.Ip, host_6.Intf.Ip))
	time.Sleep(100000000)


     	fmt.Println("Test 5 (Synchronized ARP Request) ======================================")
	host_7 := NewHost(IP{10, 0, 0, 7}, MAC{0, 0, 0, 0, 0, 7})
	go host_7.PowerOn()
	host_8 := NewHost(IP{10, 0, 0, 8}, MAC{0, 0, 0, 0, 0, 8})
	go host_8.PowerOn()
	swt_4 := NewSwitch(2)
	go swt_4.PowerOn()
	swt_5 := NewSwitch(2)
	go swt_5.PowerOn()
	Connect(&(host_7.Intf), &(swt_4.Ports[0]))
	Connect(&(host_8.Intf), &(swt_5.Ports[0]))
	Connect(&(swt_4.Ports[1]), &(swt_5.Ports[1]))
	host_5.SendArpRequest(host_6.Intf.Ip)
	time.Sleep(100000000)

	return
}

// TODO Can use mutex on receiving channels to emulate collision avoidance at the physical layer
// (Alternatively, could use actual listening CSMA/CD-style to determine if anything has recently occurred on the channel,
//  but that might be difficult to emulate using channels)

// TODO Turn this into a real application with a meta console for creating
// and configuring hosts/routers as well as console interfaces for each

// TODO Use go concurrency and channel buffering to achieve high throughput
// on simulated switch
// Maybe even just use channels to emulate delivered packets? Make each "connection"
// a channel? All this stuff is only going to make sense if the different hardware
// is emulated in parallel as different go routines
