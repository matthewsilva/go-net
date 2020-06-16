package net

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	debug_flag = flag.Bool("DEBUG", false, "a bool")
	flag.Parse()
	os.Exit(m.Run())
}

func TestSendArpRequestAsFrame(t *testing.T) {
	fmt.Println("Test 1 ======================================")
	host_1 := NewHost(IP{10, 0, 0, 1}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 1})
	go host_1.PowerOn()
	host_2 := NewHost(IP{10, 0, 0, 2}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 2})
	go host_2.PowerOn()
	Connect(&(host_1.Intf), &(host_2.Intf))
	host_1.SendFrame(NewArpRequestFrame(host_1.Intf.Mac, host_1.Intf.Ip, host_2.Intf.Ip))
	time.Sleep(100000000)
	fmt.Println(host_1)
}

func TestSwitches(t *testing.T) {
	fmt.Println("Test 2 ======================================")
	host_3 := NewHost(IP{10, 0, 0, 3}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 3})
	go host_3.PowerOn()
	host_4 := NewHost(IP{10, 0, 0, 4}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 4})
	go host_4.PowerOn()
	swt_1 := NewSwitch(4)
	go swt_1.PowerOn()
	Connect(&(host_3.Intf), &(swt_1.Ports[0]))
	Connect(&(host_4.Intf), &(swt_1.Ports[1]))
	host_3.SendFrame(NewArpRequestFrame(host_3.Intf.Mac, host_3.Intf.Ip, host_4.Intf.Ip))
}
func TestSendToDisconnectedHost(t *testing.T) {
	fmt.Println("Test 3 ======================================")
	// Should get no response
	host_1 := NewHost(IP{10, 0, 0, 1}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 1})
	go host_1.PowerOn()
	host_2 := NewHost(IP{10, 0, 0, 2}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 2})
	go host_2.PowerOn()
	host_1.SendFrame(NewArpRequestFrame(host_1.Intf.Mac, host_1.Intf.Ip, host_2.Intf.Ip))
}
func TestMultipleSwitches(t *testing.T) {
	fmt.Println("Test 4 (Two Switches) ======================================")
	host_5 := NewHost(IP{10, 0, 0, 5}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 5})
	go host_5.PowerOn()
	host_6 := NewHost(IP{10, 0, 0, 6}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 6})
	go host_6.PowerOn()
	swt_2 := NewSwitch(2)
	go swt_2.PowerOn()
	swt_3 := NewSwitch(2)
	go swt_3.PowerOn()
	Connect(&(host_5.Intf), &(swt_2.Ports[0]))
	Connect(&(host_6.Intf), &(swt_3.Ports[0]))
	Connect(&(swt_2.Ports[1]), &(swt_3.Ports[1]))
	host_5.SendFrame(NewArpRequestFrame(host_5.Intf.Mac, host_5.Intf.Ip, host_6.Intf.Ip))
}
func TestSendArpRequest(t *testing.T) {
	fmt.Println("Test 5 (Synchronized ARP Request) ======================================")
	host_7 := NewHost(IP{10, 0, 0, 7}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 7})
	go host_7.PowerOn()
	host_8 := NewHost(IP{10, 0, 0, 8}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 8})
	go host_8.PowerOn()
	swt_4 := NewSwitch(2)
	go swt_4.PowerOn()
	swt_5 := NewSwitch(2)
	go swt_5.PowerOn()
	Connect(&(host_7.Intf), &(swt_4.Ports[0]))
	Connect(&(host_8.Intf), &(swt_5.Ports[0]))
	Connect(&(swt_4.Ports[1]), &(swt_5.Ports[1]))
	SendArpRequest(host_8.Intf.Ip, host_7, &(host_7.Intf))
}
func TestPingOnLAN(t *testing.T) {
	fmt.Println("Test 6 (Echo Request / Ping) ======================================")
	host_9 := NewHost(IP{10, 0, 0, 9}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 9})
	go host_9.PowerOn()
	host_10 := NewHost(IP{10, 0, 0, 10}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 10})
	go host_10.PowerOn()
	swt_6 := NewSwitch(2)
	go swt_6.PowerOn()
	swt_7 := NewSwitch(2)
	go swt_7.PowerOn()
	Connect(&(host_9.Intf), &(swt_6.Ports[0]))
	Connect(&(host_10.Intf), &(swt_7.Ports[0]))
	Connect(&(swt_6.Ports[1]), &(swt_7.Ports[1]))
	host_9.Ping(host_10.Intf.Ip)
	host_9.Ping(host_10.Intf.Ip)
}
func TestPingThroughRouter(t *testing.T) {
	fmt.Println("Test 7 (Echo Request / Ping On Different Subnet) ======================================")
	host_11 := NewHost(IP{12, 0, 0, 11}, SubnetMask{255, 255, 255, 0}, IP{12, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 11})
	go host_11.PowerOn()
	host_12 := NewHost(IP{10, 0, 0, 12}, SubnetMask{255, 255, 255, 0}, IP{10, 0, 0, 255}, MAC{0, 0, 0, 0, 0, 12})
	go host_12.PowerOn()
	swt_8 := NewSwitch(2)
	go swt_8.PowerOn()
	swt_9 := NewSwitch(2)
	go swt_9.PowerOn()
	router_1 := NewRouter(2, []IP{IP{12, 0, 0, 255}, IP{10, 0, 0, 255}}, []SubnetMask{SubnetMask{255, 255, 255, 0}, SubnetMask{255, 255, 255, 0}}, []MAC{MAC{0, 0, 0, 0, 0, 250}, MAC{0, 0, 0, 0, 0, 251}})
	go router_1.PowerOn()
	Connect(&(host_11.Intf), &(swt_8.Ports[0]))
	Connect(&(host_12.Intf), &(swt_9.Ports[0]))
	Connect(&(swt_8.Ports[1]), &(router_1.Ports[0]))
	Connect(&(swt_9.Ports[1]), &(router_1.Ports[1]))
	host_11.Ping(host_12.Intf.Ip)
}
