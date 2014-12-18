package main

import (
	"fmt"
	"github.com/googollee/go-socket.io"
	"github.com/tatsushid/go-fastping"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type response struct {
	url   string
	addr  string
	rtt   time.Duration
	epoch int64
}

func main() {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	var p *fastping.Pinger
	p = StartPingEmitter("google.com", server)

	server.On("connection", func(so socketio.Socket) {
		// Store this by connection ID or something
		log.Println("on connection")
		so.Join("chat")
		so.On("chat message", func(msg string) {
			fmt.Println(msg)
			switch {
			case msg == "start ping":
				fmt.Print("Start Ping (Already Started)")
			case msg == "stop ping":
				fmt.Print("Stop Ping")
				p.Stop()
			}
			log.Println("emit:", so.Emit("chat message", msg))
			so.BroadcastTo("chat", "chat message", msg)
		})
		so.On("disconnection", func() {
			log.Println("on disconnect")
		})
	})
	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./public")))
	log.Println("Serving at localhost:5000...")
	log.Fatal(http.ListenAndServe(":5000", nil))
}

func StartPingEmitter(url string, server *socketio.Server) *fastping.Pinger {
	p := fastping.NewPinger()
	netProto := "ip4:icmp"
	if strings.Index(url, ":") != -1 {
		netProto = "ip6:ipv6-icmp"
	}
	ra, err := net.ResolveIPAddr(netProto, url)
	if err != nil {
		return p
	}
	results := make(map[string]*response)
	results[ra.String()] = nil
	p.AddIPAddr(ra)
	onRecv, onIdle := make(chan *response), make(chan bool)
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		onRecv <- &response{url: url, addr: addr.String(), rtt: t, epoch: time.Now().Unix()}
	}
	p.OnIdle = func() {
		onIdle <- true
	}

	//p.MaxRTT = time.Second
	p.MaxRTT = time.Millisecond * 250
	p.RunLoop()
	go func() {
		for {
			select {
			case res := <-onRecv:
				if _, ok := results[res.addr]; ok {
					results[res.addr] = res
				}
			case <-onIdle:
				for host, r := range results {
					if r == nil {
						fmt.Printf("%s : unreachable %v\n", host, time.Now())
						server.BroadcastTo("chat", "ping message", "0")
					} else {
						msg := fmt.Sprintf("%s : %v %v\n", host, r.rtt, time.Now())
						fmt.Print(msg)
						server.BroadcastTo("chat", "ping message", strings.Replace(r.rtt.String(), "ms", "", -1))
					}
					results[host] = nil
				}
			// case <-done:
			// 	fmt.Println("HERE")
			// 	p.Stop()
			case <-p.Done():
				if err = p.Err(); err != nil {
					fmt.Println("Ping failed:", err)
				}
				break
			}
		}
	}()
	return p
}

func fastpinger(url string, onRecv chan<- response) {
	p := fastping.NewPinger()
	netProto := "ip4:icmp"
	if strings.Index(url, ":") != -1 {
		netProto = "ip6:ipv6-icmp"
	}
	ra, err := net.ResolveIPAddr(netProto, url)
	if err != nil {
		return
	}
	p.AddIPAddr(ra)
	//onRecv := make(chan *response)
	onIdle := make(chan bool)
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		epoch := time.Now().Unix()
		fmt.Println("here")
		onRecv <- response{url: url, addr: addr.String(), rtt: t, epoch: epoch}
		fmt.Println(t)
		p.Stop()
	}
	p.OnIdle = func() {
		onIdle <- true
	}
	p.MaxRTT = time.Second
	p.RunLoop()
	fmt.Println("here")
	return
	// fmt.Println(res.rtt)
	// fmt.Printf("ping.%s %s %v\n", url,  strings.Replace(r.rtt.String(), "ms","", -1), time.Now().Unix())
}
