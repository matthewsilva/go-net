package main

import "github.com/matthewsilva/go-net/Net"

import (
	   "bufio"
	   "fmt"
	   "os"
)


type NetworkConsole struct {
	 Hosts []net.Host
	 Switches []net.Switch
	 Routers []net.Router	 
}

func InterpretCommand(command string) {
	 fmt.Print("Command was", command)
}

func RunNetworkConsole() {
	 fmt.Println("$ Type \"help\" for a list of commands")
	 reader := bufio.NewReader(os.Stdin)
	 for {
	 	 fmt.Print("$ ")
		 str, err := reader.ReadString('\n')
		 if err != nil {
		 	fmt.Println("Error received during command input:", err)	
		 } else {
		 		 InterpretCommand(str)
		}
	}
}

func main() {
	 RunNetworkConsole()
}