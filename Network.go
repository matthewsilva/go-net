package main

import (
       "errors"
       "fmt"
)

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
var broadcast_mac MAC = MAC{0xFF,0xFF,0xFF,0xFF,0xFF,0xFF}
func (mac MAC) String() string {
     return fmt.Sprintf("%x.%x.%x.%x.%x.%x",mac[0],mac[1],mac[2],mac[3],mac[4],mac[5])
}

type Opcode int

const (
      ARP_REQUEST Opcode = iota
      ARP_REPLY Opcode = iota
)

type FramePayload struct {
     Op Opcode
}

type Frame struct {
     destMac MAC
     sourceMac MAC
     payload FramePayload
}

type Interface struct {
     Name string
     Ip IP
     Mac MAC
     // Denotes the host associated with this interface
     Owner *Host
     // Denotes where this interface is connected (nil if no connection)
     Connected_to *Interface
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

type Host struct {
     // TODO Need list of interfaces
     // TODO add hostname
     Intf Interface
     arp_table map[IP]MAC
}
func (host *Host) String() string {
     return host.Intf.String() + "," + fmt.Sprint(host.arp_table)
}

func NewHost(ip IP, mac MAC) *Host {
     var new_host Host = Host{Interface{"eth0"/*TODO*/,ip, mac, nil, nil}, make(map[IP]MAC)}
     new_host.Intf.Owner = &new_host
     return &new_host
}

/*
type Switch struct {
     Name string
     Ports []Interface     
}
*/

// This is a meta function that allows the user to simulate
// connecting two hosts on the network by ethernet
func Connect(intf_1, intf_2 *Interface) error {
   if (intf_1 == nil && intf_2 == nil) {
      return errors.New("Connect(...): intf_1 and intf_2 are nil")
   } else if (intf_1 == nil) {
      return errors.New("Connect(...): intf_1 is nil")
   } else if (intf_2 == nil) {
      return errors.New("Connect(...): intf_2 is nil")
   }

   intf_1.Connected_to = intf_2
   intf_2.Connected_to = intf_1
   return nil
}

// TODO this should return a packet response??
/*
func (host *Host) SendPacket(destIp IP) {
     if mac, found := host.arp_table[destIP]; found {
     	host.SendFrame(mac,destIp)
     } else {
        host.SendFrame(broadcast,destIp)
     }
}
*/

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

func (host *Host) ReceiveFrame(frame Frame) {
     if host != nil {
     	fmt.Println("ReceiveFrame(...): Receiving frame at ", frame.destMac)
     	if frame.destMac == host.Intf.Mac {
     	   fmt.Printf("ReceiveFrame(...): Frame reached destination, payload was %v \n", frame.payload)
	   if frame.payload.Op == ARP_REQUEST {
	      response := Frame{frame.sourceMac, frame.destMac, FramePayload{ARP_REPLY}}
	      host.SendFrame(response)
	   }
     	}
     }
}

func (host *Host) SendFrame(frame Frame) {
     if host != nil {
     	// Send the frame to wherever our interface is connected to
     	host.Intf.Connected_to.Owner.ReceiveFrame(frame)
     }
}

func main() {
     host_1 := NewHost(IP{10,0,0,1},MAC{0,0,0,0,0,0})
     host_2 := NewHost(IP{10,0,0,2},MAC{0,0,0,0,0,1})
     fmt.Println(host_1)
     Connect(&(host_1.Intf), &(host_2.Intf))
     host_1.SendFrame(Frame{host_2.Intf.Mac, host_1.Intf.Mac, FramePayload{ARP_REQUEST}})
     return
}

// TODO Turn this into a real application with a meta console for creating
// and configuring hosts/routers as well as console interfaces for each

// TODO Use go concurrency and channel buffering to achieve high throughput
// on simulated switch
// Maybe even just use channels to emulate delivered packets? Make each "connection"
// a channel? All this stuff is only going to make sense if the different hardware
// is emulated in parallel as different go routines