package net

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

var debug_flag *bool

type IP [4]uint8
type SubnetMask [4]uint8

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
	HardwareType       uint16
	ProtocolType       uint16
	HardwareAddrLen    uint8
	ProtocolAddrLen    uint8
	OperationType      Opcode
	SourceHardwareAddr MAC // TODO this should be uint48
	SourceProtocolAddr IP  // TODO this should be uint32
	DestHardwareAddr   MAC
	DestProtocolAddr   IP
}

type IPv4Protocol uint8

const (
	ICMP IPv4Protocol = 0x01
	TCP  IPv4Protocol = 0x06
	UDP  IPv4Protocol = 0x11
)

type IPv4Packet struct {
	VersionAndIHL          uint8
	DSCPandECN             uint8
	TotalLength            uint16
	Identification         uint16
	FlagsAndFragmentOffset uint16
	TimeToLive             uint8 // TODO See https://en.wikipedia.org/wiki/IPv4#TTL (This means that every single packet sent that expects a response should also expect an ICMP Time Exceeded message. This can be done by including it in the lambda function that the Session is created with)
	// TODO Alternatively, golang timers could be used to automatically notify on a the channel after a set amount of time (https://gobyexample.com/timers). These could be set up by the waiting program before beginning to wait on the channel such that there would be a guarantee that they would come out of the wait eventually. Be sure to devise a way to know whether the channel actually received a response or just timed out and got a phony timer response
	Protocol       IPv4Protocol
	HeaderChecksum uint16
	SourceIp       IP
	DestIp         IP
	Data           interface{}
}

type ICMPType uint8

const (
	EchoReply              ICMPType = 0
	DestinationUnreachable ICMPType = 3
	EchoRequest            ICMPType = 8
	// TODO TimeExceeded ICMPType = 11
)

type ICMPDatagram struct {
	PacketType ICMPType
	ICMPCode   uint8
	Checksum   uint16
}

type EtherType uint16

const (
	IPv4 EtherType = 0x0800
	ARP  EtherType = 0x0806
)

type Frame struct {
	destMac   MAC
	sourceMac MAC
	etherType EtherType
	payload   FramePayload // TODO Need to find a way to make this a variably sized polymorphic struct
	// It's also possible to just make it a byte field but that wouldn't be very abstract
}

func (frame Frame) String() string {
	str := fmt.Sprintf("Dest:%v, Source:%v, EtherType:%v,", frame.destMac, frame.sourceMac, frame.etherType)
	switch frame.etherType {
	case ARP:
		if payload, correct_type := frame.payload.Data.(ARPPayload); !correct_type {
			str += "Error:Incorrect payload type for etherType"
		} else {
			str += fmt.Sprintf("ARPPayload{OperationType: %v}", payload.OperationType)
		}
	case IPv4:
		str += fmt.Sprintf("IPv4Packet{TODO, Show contents / packet type}")
	default:
		str += "Error:Unknown ethertype"
	}
	return str

}

type Interface struct {
	Name           string
	Ip             IP
	Mask           SubnetMask
	DefaultGateway IP
	Mac            MAC
	// Denotes where this interface is connected (nil if no connection)
	Connection   chan Frame // channels represent an ethernet connection
				 	  		// TODO Investigate turning these channels/Frames into structs that implement the reader interface
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
	Response        chan Frame
}

type Host struct {
	// TODO Need list of interfaces
	// TODO add hostname
	Intf          Interface
	arp_table     map[IP]MAC // TODO Need to add mutex for arp_table
	Sessions      []Session
	SessionsMutex sync.Mutex
	ConsoleInput       []string
	ConsoleOutput       []string
}

func (host *Host) String() string {
	return host.Intf.String() + "," + fmt.Sprint(host.arp_table)
}

func NewHost(ip IP, mask SubnetMask, default_gateway IP, mac MAC) *Host {
	return &Host{Interface{Name: "eth0" /*TODO*/, Ip: ip, Mask: mask, DefaultGateway: default_gateway, Mac: mac, Connection: make(chan Frame), Connected_to: nil}, make(map[IP]MAC), nil, sync.Mutex{}, nil, nil}
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
		ip := IP{0, 0, 0, 0} /*TODO*/
		mac := MAC{uint8(rand.Intn(256)), uint8(rand.Intn(256)), uint8(rand.Intn(256)), uint8(rand.Intn(256)), uint8(rand.Intn(256)), uint8(rand.Intn(256))}

		swt.Ports[i] = Interface{Name: "eth0" /*TODO*/, Ip: ip, Mac: mac, Connection: make(chan Frame), Connected_to: nil}
	}
	swt.Mac_table = make(map[MAC]int)
	return &swt
}

func (swt *Switch) PowerOn() {
	var cases []reflect.SelectCase = make([]reflect.SelectCase, len(swt.Ports))
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

type Router struct {
	Name          string
	Ports         []Interface
	arp_table     map[IP]MAC
	Routing_table int // TODO
	Sessions      []Session
	SessionsMutex sync.Mutex
}

func NewInterface(mac MAC, ip IP) Interface {
	return Interface{Name: "eth0" /*TODO*/, Ip: ip, Mac: mac, Connection: make(chan Frame), Connected_to: nil}
}

func NewRouter(num_ports int, ip_list []IP, mask_list []SubnetMask, mac_list []MAC) *Router {
	var router Router
	router.Name = "router_0"
	router.Ports = make([]Interface, num_ports)
	for i := range router.Ports {
		ip := ip_list[i]
		mask := mask_list[i]
		mac := mac_list[i]
		router.Ports[i] = Interface{Name: "eth0" /*TODO*/, Ip: ip, Mask: mask, Mac: mac, Connection: make(chan Frame), Connected_to: nil}
	}
	router.arp_table = make(map[IP]MAC)
	return &router
}

func (router *Router) PowerOn() {
	var cases []reflect.SelectCase = make([]reflect.SelectCase, len(router.Ports))
	for i := range router.Ports {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(router.Ports[i].Connection)}
	}

	for {
		// reflect select
		port_num, recv_frame, _ := reflect.Select(cases)
		// ReceiveFrame must be run as a goroutine because there may be blocking calls initiated by the received frame
		// that require another incoming frame to stop blocking (e.g. ReceiveFrame may initiate an ARP request)
		go ReceiveFrame(router, recv_frame.Interface().(Frame), &(router.Ports[port_num]))
	}
}

func (router *Router) ReceiveIPv4Packet(frame Frame, packet IPv4Packet, receiving_intf *Interface) {
	// Rudimentary automatic routing (Only allows directly connected networks to connect)
	// TODO This will not work if we have two routers in series, Need to add routing table
	if packet.DestIp != receiving_intf.Ip {
		for i := range router.Ports {
			if SameSubnet(router.Ports[i].Ip, packet.DestIp, router.Ports[i].Mask) {
				router.SendPacket(packet, &(router.Ports[i]))
			}
		}
		//router.SendPacket(packet, default_route_intf)
	} else {
	}

}

func (router *Router) SendPacket(packet IPv4Packet, sourceIntf *Interface) {

	if _, found := router.arp_table[packet.DestIp]; !found {
		if *debug_flag {
			fmt.Println("SendPacket(...): Router", router, "sending ARP Request...")
		}
		SendArpRequest(packet.DestIp, router, sourceIntf)
		if *debug_flag {
			fmt.Println("SendPacket(...): Router", router, "received ARP Reply...")
		}
	}
	if mac, found := router.arp_table[packet.DestIp]; found {
		SendFrame(Frame{destMac: mac, sourceMac: sourceIntf.Mac, etherType: IPv4, payload: FramePayload{packet}}, sourceIntf)
	} else {
		fmt.Println("SendPacket(...): The ARP Request probably timed out, no MAC address found for destination IP")
	}
}

type IpCapableDevice interface {
	// The below four methods allow us to reuse ARP code for both hosts and routers
	ArpTable() *map[IP]MAC
	GetSessions() *[]Session
	SetSessions([]Session)
	GetSessionsMutex() *sync.Mutex

	// Routers and hosts will use mostly different code for receiving non-ARP packets
	ReceiveIPv4Packet(frame Frame, packet IPv4Packet, receiving_intf *Interface) // TODO
}

func (router *Router) ArpTable() *map[IP]MAC          { return &(router.arp_table) }
func (router *Router) GetSessions() *[]Session        { return &(router.Sessions) }
func (router *Router) SetSessions(sessions []Session) { router.Sessions = sessions }
func (router *Router) GetSessionsMutex() *sync.Mutex  { return &(router.SessionsMutex) }
func (host *Host) ArpTable() *map[IP]MAC              { return &(host.arp_table) }
func (host *Host) GetSessions() *[]Session            { return &(host.Sessions) }
func (host *Host) SetSessions(sessions []Session)     { host.Sessions = sessions }
func (host *Host) GetSessionsMutex() *sync.Mutex      { return &(host.SessionsMutex) }

func HandleArpFrame(dev IpCapableDevice, frame Frame, receiving_intf *Interface) {
	if payload, correct_type := frame.payload.Data.(ARPPayload); !correct_type {
		fmt.Printf("ReceiveFrame(...): Error, FramePayload was incorrect type %t\n", frame.payload.Data)
	} else {
		if payload.DestProtocolAddr == receiving_intf.Ip {
			switch payload.OperationType {
			case ARP_REQUEST:
				(*dev.ArpTable())[payload.SourceProtocolAddr] = payload.SourceHardwareAddr
				reply := NewArpReplyFrame(receiving_intf.Mac, receiving_intf.Ip, payload.SourceHardwareAddr, payload.SourceProtocolAddr)
				SendFrame(reply, receiving_intf)
			case ARP_REPLY:
				if *debug_flag {
				   fmt.Println("ReceiveFrame(...): Received ARP_REPLY")
				}
				// Add Reply to arp_table regardless of whether this arp reply was solicited or unsolicited
				(*dev.ArpTable())[payload.SourceProtocolAddr] = payload.SourceHardwareAddr
				// Notify host that an ARP reply was received (if it is waiting for one)
				SendFrameToMatchingSessions(dev, frame, true)
			}
		}
	}
}

// TODO Need to modularize the ReceiveFrame method to reduce ARP code duplication between hosts and routers.
// Every interface on a router needs to have its own ARP table, so maybe there could just be an update ARP table method that takes
// an interface and an ARP frame
func ReceiveFrame(dev IpCapableDevice, frame Frame, receiving_intf *Interface) {
	if *debug_flag {
		fmt.Println("ReceiveFrame(...): Device", dev, "receiving frame", frame)
	}
	if frame.destMac == receiving_intf.Mac || frame.destMac == broadcast_mac {
		switch frame.etherType {
		case ARP: // Put each case into its own method
			if *debug_flag {
				fmt.Println("ReceiveFrame(...): Receiving ARP frame...")
			}
			HandleArpFrame(dev, frame, receiving_intf)
		case IPv4:
			if packet, correct_type := frame.payload.Data.(IPv4Packet); !correct_type {
				fmt.Printf("ReceiveFrame(...): Error, FramePayload was incorrect type for EtherType of %v\n", frame.etherType)
			} else {
				if *debug_flag {
					fmt.Println("ReceiveFrame(...): Received Frame with IPv4 Payload")
				}
				dev.ReceiveIPv4Packet(frame, packet, receiving_intf)
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
		DestHardwareAddr:   destMac,
		DestProtocolAddr:   destIp}
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

func NewICMPEchoRequest(destIp IP, sourceIp IP) IPv4Packet {
	return NewICMPPacket(destIp, sourceIp, EchoRequest)
}
func NewICMPEchoReply(destIp IP, sourceIp IP) IPv4Packet {
	return NewICMPPacket(destIp, sourceIp, EchoReply)
}
func NewICMPPacket(destIp, sourceIp IP, icmpType ICMPType) IPv4Packet {
	var packet IPv4Packet = IPv4Packet{Protocol: ICMP, SourceIp: sourceIp, DestIp: destIp, Data: ICMPDatagram{PacketType: icmpType}}
	return packet
}

func (host *Host) Ping(destIp IP) {
	var echo_reply_chan chan Frame = make(chan Frame)
	icmp_request_session := Session{
		func(frame Frame) bool {
			if payload, payload_type_good := frame.payload.Data.(IPv4Packet); payload_type_good &&
				payload.Protocol == ICMP &&
				payload.SourceIp == destIp &&
				payload.DestIp == host.Intf.Ip {
				if icmp_datagram, data_type_good := payload.Data.(ICMPDatagram); data_type_good &&
					icmp_datagram.PacketType == EchoReply {
					return true
				}
			}
			return false
		},
		echo_reply_chan}
	host.SessionsMutex.Lock()
	host.Sessions = append(host.Sessions, icmp_request_session) // A session is composed of a Frame comparison function and a channel to notify on if that function returns true
	host.SessionsMutex.Unlock()
	send_time := time.Now()
	host.SendPacket(NewICMPEchoRequest(destIp, host.Intf.Ip))
	// Wait until the ARP request has been answered
	<-echo_reply_chan // Nothing needs to be done with the output so far
	ping_time := time.Since(send_time)
	ping_message := fmt.Sprint("Ping to", destIp, "took", ping_time)
	if *debug_flag {
	   fmt.Println("SendArpRequest(...):", ping_message)
	}
	host.ConsoleOutput = append(host.ConsoleOutput, ping_message)
}

func SameSubnet(ip1, ip2 IP, mask SubnetMask) bool {
	for i := range ip1 {
		if ip1[i]&mask[i] != ip2[i]&mask[i] {
			return false
		}
	}
	return true
}

func (host *Host) SendPacket(packet IPv4Packet) {

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
	var NextHopIp IP = packet.DestIp
	if !SameSubnet(host.Intf.Ip, packet.DestIp, host.Intf.Mask) {
		NextHopIp = host.Intf.DefaultGateway
		if *debug_flag {
			fmt.Println("SendPacket(...): Sending packet to default gateway...")
		}
	}
	if _, found := host.arp_table[NextHopIp]; !found {
		if *debug_flag {
			fmt.Println("SendPacket(...): Host", host, "sending ARP Request...")
		}
		SendArpRequest(NextHopIp, host, &(host.Intf))
		if *debug_flag {
			fmt.Println("SendPacket(...): Host", host, "received ARP Reply...")
		}
	}
	if mac, found := host.arp_table[NextHopIp]; found {
		SendFrame(Frame{destMac: mac, sourceMac: host.Intf.Mac, etherType: IPv4, payload: FramePayload{packet}}, &(host.Intf))
	} else {
		fmt.Println("SendPacket(...): The ARP Request probably timed out, no MAC address found for destination IP")
	}

}

func SendArpRequest(destIp IP, dev IpCapableDevice, sourceIntf *Interface) {
	var arp_reply_chan chan Frame = make(chan Frame)
	arp_request_session := Session{
		func(frame Frame) bool {
			if payload, correct_type := frame.payload.Data.(ARPPayload); frame.destMac == sourceIntf.Mac && correct_type &&
				payload.SourceProtocolAddr == destIp &&
				payload.DestProtocolAddr == sourceIntf.Ip &&
				payload.DestHardwareAddr == sourceIntf.Mac {
				return true
			} else {
				return false
			}
		},
		arp_reply_chan}
	dev.GetSessionsMutex().Lock()
	dev.SetSessions(append((*dev.GetSessions()), arp_request_session)) // A session is composed of a Frame comparison function and a channel to notify on if that function returns true
	dev.GetSessionsMutex().Unlock()
	SendFrame(NewArpRequestFrame(sourceIntf.Mac, sourceIntf.Ip, destIp), sourceIntf)
	// Wait until the ARP request has been answered
	<-arp_reply_chan // Nothing needs to be done with the output frame from the channel because ARP replies get handled automatically in ReceiveFrame such that we are able to accomodate unsolicited ARP announcements
	if *debug_flag {
		fmt.Println("SendArpRequest(...): Received ARP Reply frame from Channel")
	}
}

func RemoveFrameFromSlice(slice []Session, index_to_remove int) []Session {
	slice = append(slice[:index_to_remove], slice[index_to_remove+1:]...)
	return slice
}

func SendFrameToMatchingSessions(dev IpCapableDevice, frame Frame, removeSessions bool) {
	dev.GetSessionsMutex().Lock()
	for i, session := range *dev.GetSessions() {
		if session.IsMatchingFrame(frame) {
			if *debug_flag {
				fmt.Println("SendFrameToMatchingSessions(...): Sending frame to matching channel")
			}
			session.Response <- frame
			if removeSessions {
				dev.SetSessions(RemoveFrameFromSlice((*dev.GetSessions()), i))
			}
		}
	}
	dev.GetSessionsMutex().Unlock()
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
	var cases []reflect.SelectCase = make([]reflect.SelectCase, 1)
	cases[0] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(host.Intf.Connection)}

	for {
		// reflect select
		_, recv_frame, _ := reflect.Select(cases)
		// ReceiveFrame must be run as a goroutine because there may be blocking calls initiated by the received frame
		// that require another incoming frame to stop blocking (e.g. ReceiveFrame may initiate an ARP request)
		go ReceiveFrame(host, recv_frame.Interface().(Frame), &(host.Intf))
	}
}

/*
func (host *Host) ReceiveFrame(frame Frame, intf *Interface) {
	if host != nil {
		if *debug_flag {
			fmt.Println("ReceiveFrame(...): Host", host, "receiving frame", frame)
		}
		if frame.destMac == host.Intf.Mac || frame.destMac == broadcast_mac {
			switch frame.etherType {
			case ARP: // Put each case into its own method
				HandleArpFrame(host, frame, intf)
			case IPv4:
				if packet, correct_type := frame.payload.Data.(IPv4Packet); !correct_type {
					fmt.Printf("ReceiveFrame(...): Error, FramePayload was incorrect type for EtherType of %v\n", frame.etherType)
				} else {
					if *debug_flag {
						fmt.Println("ReceiveFrame(...): Received Frame with IPv4 Payload")
					}
					host.ReceiveIPv4Packet(frame, packet, intf)
				}
			}
		}
	}
}
*/
func SendFrame(frame Frame, intf *Interface) {
	if *debug_flag {
		fmt.Println("SendFrame(...): Sending frame", frame)
	}
	// Send the frame out of our interface to wherever our interface is connected to
	intf.Send(frame)
}

// Convenience method for hosts since they only have one interface
func (host *Host) SendFrame(frame Frame) {
	if host != nil {
		if *debug_flag {
			fmt.Println("SendFrame(...): Host", host, "sending frame", frame)
		}
		// Send the frame out of our interface to wherever our interface is connected to
		host.Intf.Send(frame)
	}
}

func (host *Host) ReceiveIPv4Packet(containing_frame Frame, packet IPv4Packet, intf *Interface) {
	if packet.DestIp == intf.Ip {
		switch packet.Protocol {
		case ICMP:
			if icmp_datagram, data_type_good := packet.Data.(ICMPDatagram); data_type_good {
				switch icmp_datagram.PacketType {
				case EchoReply:
					if *debug_flag {
						fmt.Println("ReceiveIPv4Packet(...): Received ICMP echo reply")
					}
					// Notify host that a reply to an echo request was received (if it is waiting for one)
					SendFrameToMatchingSessions(host, containing_frame, true)

				case EchoRequest:
					if *debug_flag {
						fmt.Println("ReceiveIPv4Packet(...): Received ICMP echo request")
					}

					// Send a response back to the host
					host.SendPacket(NewICMPEchoReply(packet.SourceIp, host.Intf.Ip))

				default:
					fmt.Println("ReceiveIPv4Packet(...): Unknown Protocol type", packet.Protocol, "in IPv4Packet")
				}
			}
		}
	}
}

// TODO Everything more above and including an ARP request should probably be run in parallel because it needs to wait on the response after sending (Unless that logic can be worked into the ReceiveFrame method)
// TODO The network actually could be deterministic even with parallel devices, if everything is calculated in steps (with a separate sending stage most likely). e.g. Every network tick, all devices receive once (there will be a barrier at the bottom of the infinite receive for loop, and a default case for no receives), process the packets, and prepare to send. Then they all wait on a barrier to send, then they all send. Only problem is that sends would all now need to be run as go routines because otherwise there would be deadlock on every send (if a switch wants to send, it will wait on the channel until somone receives, but all the other devices will be waiting at a barrier for their next read, and will never become ready to read. This could be fixed with channel buffers that don't block when writing, but that can come later

// TODO To allow for sessions (and anything else that requires continuity between a send and subsequent receives), there should probably be a stack of "receivers" for running processes that need to act on specific received inputs that are related to packets they sent out earlier
// One good example of the above is when a host wants to send a packet but needs to do an ARP request first. It needs to wait for the ARP request to be returned before it can correctly address the packet it wants to send across the network

func main() {
	 fmt.Println("Great!")
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
