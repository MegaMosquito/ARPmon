/*
  arpmon  Implements a REST API to query ARP scanning results

  Written by Glen Darling, April 2021.
*/

package main

import (
	"context"
	"fmt"
	"github.com/j-keck/arping"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const ADDRESS_FIRST int = 1
const ADDRESS_LAST int = 254
const WORKER_SLEEP_SEC = 6
const ARP_TIMEOUT time.Duration = (6 * time.Second)
const NOT_FOUND string = ""

const GPS_DEBUG = false

func debug(formatStr string, args ...interface{}) {
	if GPS_DEBUG {
		log.Printf("DEBUG: "+formatStr, args...)
	}
}

type Server struct {

	// Threading
	wg                   sync.WaitGroup
	mutex                sync.RWMutex
	ch                   chan int
	number_of_goroutines int

	// REST server
	url_base string

	// LAN monitoring
	prefix   string
	my_octet int
	my_mac   string
	hosts    map[int]string
}

func ServerFactory(url_base, cidr, my_ipv4, my_mac string, n int) *Server {
	arping.SetTimeout(ARP_TIMEOUT)
	var hosts = make(map[int]string)
	for i := ADDRESS_FIRST; i <= ADDRESS_LAST; i++ {
		hosts[i] = NOT_FOUND
	}
	octets := strings.Split(my_ipv4, ".")
	octet, err := strconv.Atoi(octets[len(octets)-1])
	if err != nil {
		log.Fatal(err)
	}
	hosts[octet] = my_mac
	s := &Server{hosts: hosts}
	s.mutex.Lock()
	s.number_of_goroutines = n
	s.ch = make(chan int, s.number_of_goroutines)
	s.url_base = url_base
	s.prefix = strings.Join(strings.Split(cidr, ".")[:3], ".") + "."
	s.my_octet = octet
	s.my_mac = my_mac
	s.mutex.Unlock()
	segment_size := 256 / s.number_of_goroutines
	for i := 0; i < s.number_of_goroutines; i++ {
		min := i * segment_size
		if min < ADDRESS_FIRST {
			min = ADDRESS_FIRST
		}
		max := (i+1)*segment_size - 1
		if max > ADDRESS_LAST {
			max = ADDRESS_LAST
		}
		s.mutex.Lock()
		s.wg.Add(1)
		go s.work(min, max)
		s.mutex.Unlock()
	}
	return s
}

func (s *Server) arp_ping(octet int, ip string) {
	debug("--> arp-ping(%d, %s)\n", octet, ip)
	// Do an ARP probe and wait for response
	// arping.EnableVerboseLog()
	mac, msec, err := arping.Ping(net.ParseIP(ip))
	if err == arping.ErrTimeout {
		debug("<-- %s  [timeout]\n", ip)
		s.mutex.Lock()
		s.hosts[octet] = NOT_FOUND
		s.mutex.Unlock()
	} else if err != nil {
		// log.Printf("Ignoring error from arping.Ping():\n   %v", err)
	} else {
		mac_str := strings.ToUpper(fmt.Sprint(mac))
		debug("<-- %s (%s) %d usec\n", ip, mac_str, msec/1000)
		s.mutex.Lock()
		s.hosts[octet] = mac_str
		s.mutex.Unlock()
	}
}

func (s *Server) work(min int, max int) {
	debug("WORKER(%d .. %d)\n", min, max)
	defer s.wg.Done()
	s.mutex.RLock()
	my_octet := s.my_octet
	prefix := s.prefix
	s.mutex.RUnlock()
	for {
		for i := min; i <= max; i++ {
			if i != my_octet {
				ip := prefix + fmt.Sprint(i)
				s.arp_ping(i, ip)
				for j := 0; j < (2 * WORKER_SLEEP_SEC); j++ {
					select {
					case <-s.ch:
						debug("<== WORKER(%d .. %d)\n", min, max)
						return
					default:
						time.Sleep(500 * time.Millisecond)
					}
				}
			}
		}
	}
}

// Construct a response of only unique MAC addresses found, one per line.
func (s *Server) macs() string {
	var macs = make(map[string]bool)
	for i := ADDRESS_FIRST; i <= ADDRESS_LAST; i++ {
		s.mutex.RLock()
		mac := s.hosts[i]
		s.mutex.RUnlock()
		debug("MAC=%v\n", mac)
		if NOT_FOUND != mac {
			macs[mac] = true
		}
	}
	var response string = ""
	format := "%s\n"
	for mac, _ := range macs {
		response += fmt.Sprintf(format, mac)
	}
	return response
}

// Construct a CSV response of "IP,MAC" lines
func (s *Server) csv(prefix string) string {
	var response string = ""
	format := "%s,%s\n"
	for i := ADDRESS_FIRST; i <= ADDRESS_LAST; i++ {
		s.mutex.RLock()
		mac := s.hosts[i]
		s.mutex.RUnlock()
		if NOT_FOUND != mac {
			ip := prefix + fmt.Sprint(i)
			response += fmt.Sprintf(format, ip, mac)
		}
	}
	return response
}

// Construct a JSON response in this form:
//  {
//    "hosts": [
//      {
//        "ip": "x.x.x.x",
//        "mac": "xx:xx:xx:xx:xx:xx"
//      },
//      ...
//    ]
//  }
func (s *Server) json(prefix string) string {
	var response string = "{\n  \"hosts\":[\n"
	format := "    {\"ip\":\"%s\",\"mac\":\"%s\"}"
	count := 0
	for i := ADDRESS_FIRST; i <= ADDRESS_LAST; i++ {
		s.mutex.RLock()
		mac := s.hosts[i]
		s.mutex.RUnlock()
		if NOT_FOUND != mac {
			if count > 0 {
				response += ",\n"
			}
			count++
			ip := prefix + fmt.Sprint(i)
			response += fmt.Sprintf(format, ip, mac)
		}
	}
	response += "\n  ]\n}\n"
	return response
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("REQUEST: %s %v\n", r.Method, r.URL)
	s.mutex.RLock()
	prefix := s.prefix
	url_base := s.url_base
	s.mutex.RUnlock()
	url := fmt.Sprintf("%v", r.URL)
	debug(url)
	var response string
	if url_base+"/macs" == url {
		response = s.macs()
	} else if url_base+"/csv" == url {
		response = s.csv(prefix)
	} else if url_base+"/json" == url {
		response = s.json(prefix)
	} else {
		log.Fatal("Impossible URL: " + url)
	}
	_, err := fmt.Fprintf(w, response)
	if err != nil {
		log.Print(err)
	} else {
		log.Print(response)
	}
}

func get_env_or_default(name string, def string) string {
	var val string = os.Getenv(name)
	debug("ENV %s==\"%v\"\n", name, val)
	if "" == val {
		return def
	} else {
		return val
	}
}

func main() {

	// Collect configuration from the process environment
	var cidr string = get_env_or_default("CIDR", "")
	var my_ipv4 string = get_env_or_default("IPV4", "")
	var my_mac string = strings.ToUpper(get_env_or_default("MAC", ""))
	var port string = fmt.Sprintf(":%s", get_env_or_default("PORT", "1234"))
	var url_base string = get_env_or_default("URL_BASE", "/")
	var goroutines_str string = get_env_or_default("GOROUTINES", "4")
	if "" == cidr || "" == my_ipv4 || "" == my_mac {
		log.Fatal("\"CIDR\", \"IPV4\" and \"MAC\" must be set in environment.")
	}
	n, err := strconv.Atoi(goroutines_str)
	if err != nil {
		log.Fatal(err)
	}

	// Channel "sigs" will receive OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Create the Server object that does all the work
	s := ServerFactory(url_base, cidr, my_ipv4, my_mac, n)

	// Make it into an HTTP REST API server
	rest_server := &http.Server{Addr: port, Handler: http.DefaultServeMux}
	http.Handle(url_base+"/macs", s)
	http.Handle(url_base+"/csv", s)
	http.Handle(url_base+"/json", s)
	go func() {
		debug("Starting REST server at %s, with URL base \"%s\"\n", port, url_base)
		err := rest_server.ListenAndServe()
		debug("REST server has terminated.\n")
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Signal handler
	go func() {

		// Block waiting for signals
		<-sigs
		log.Println("SIGNAL!")

		// Send s.number_of_goroutines messages (each will receive one)
		for i := 0; i < s.number_of_goroutines; i++ {
			s.ch <- i
		}

		// End the web server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rest_server.Shutdown(ctx)

		log.Println("Waiting for threads to complete (please be patient)...")
	}()

	s.wg.Wait()
	log.Println("Graceful shutdown complete.")
}
