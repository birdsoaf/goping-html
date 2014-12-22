package main

import (
	"encoding/json"
	"fmt"
	"github.com/googollee/go-socket.io"
	"github.com/tatsushid/go-fastping"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Response struct {
	Url   string
	Addr  string
	Rtt   time.Duration
	Epoch int64
}

type PingManger struct {
	p       *fastping.Pinger
	results map[string]*Response
	ipTourl map[string]string
}

func main() {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	var pm *PingManger
	pinglist := make([]string, 2)
	pinglist[0] = "google.com"
	pinglist[1] = "cogentco.com"
	pm = StartPingEmitter(pinglist, server)
	// p = StartPingEmitter("google.com", server)

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
				pm.p.Stop()
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
	http.Handle("/add/{url}", handler)
	log.Println("Serving at localhost:5000...")
	log.Fatal(http.ListenAndServe(":5000", nil))
}

func AddURL(rw http.ResponseWriter, req *http.Request, pm PingManger) {

}

func StartPingEmitter(urls []string, server *socketio.Server) *PingManger {
	pm := PingManger{
		p:       fastping.NewPinger(),
		results: make(map[string]*Response),
		ipTourl: make(map[string]string),
	}
	netProto := "ip4:icmp"
	if strings.Index(urls[0], ":") != -1 {
		netProto = "ip6:ipv6-icmp"
	}
	for _, element := range urls {
		ra, err := net.ResolveIPAddr(netProto, element)
		if err != nil {
			return &pm
		}
		pm.ipTourl[ra.String()] = element
		pm.results[ra.String()] = nil
		pm.p.AddIPAddr(ra)
	}
	onRecv, onIdle := make(chan *Response), make(chan bool)
	pm.p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		onRecv <- &Response{Url: pm.ipTourl[addr.String()], Addr: addr.String(), Rtt: t, Epoch: time.Now().Unix()}
	}
	pm.p.OnIdle = func() {
		onIdle <- true
	}

	//p.MaxRTT = time.Second
	pm.p.MaxRTT = time.Millisecond * 250
	pm.p.RunLoop()
	go func() {
		for {
			select {
			case res := <-onRecv:
				if _, ok := pm.results[res.Addr]; ok {
					pm.results[res.Addr] = res
				}
			case <-onIdle:
				for host, r := range pm.results {
					if r == nil {
						fmt.Printf("%s : unreachable %v\n", host, time.Now())
						server.BroadcastTo("chat", "ping message", "0")
					} else {
						// msg := fmt.Sprintf("%s : %v %v\n", host, r.rtt, time.Now())
						// fmt.Print(msg)
						b, _ := json.Marshal(pm.results[host])
						// fmt.Println(results[host])
						// fmt.Println(string(b))
						server.BroadcastTo("chat", "ping message", string(b))
					}
					pm.results[host] = nil
				}
			// case <-done:
			// 	fmt.Println("HERE")
			// 	p.Stop()
			case <-pm.p.Done():
				break
			}
		}
	}()
	return &pm
}
