package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
)

var (
	// not the ideal place for this flag
	friendlySocketNames = flag.String("hostapd.friendly_socket_names", "", "JSON map providing friendly names for hostapd sockets (ideally the SSID)")
)

type DashServer struct {
	h *http.Server

	index  *template.Template
	static http.Handler

	hostapd *HostapdControl
	dhcp    *DHCPStore

	socketRename map[string]string
}

type Client struct {
	MAC      net.HardwareAddr
	IPs      []net.IP
	AP       string
	Hostname string
	Vendor   string
}

type Page struct {
	Clients []Client
}

func (d *DashServer) render() (*Page, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	connectedClients, err := d.hostapd.ListClients(ctx)
	if err != nil {
		return nil, err
	}

	clientC := make(chan Client)
	for _, c := range connectedClients {
		mac, err := net.ParseMAC(c.Addr)
		if err != nil {
			return nil, err
		}

		ap, ok := d.socketRename[c.SocketName]
		if !ok {
			ap = c.SocketName
		}

		go func(mac net.HardwareAddr, ap string) {
			ips, ci, err := d.dhcp.LookupDevice(ctx, mac)
			if err != nil {
				glog.Errorf("error looking up device \"%s\": %v", mac, err)
				clientC <- Client{
					MAC: mac,
				}
			} else {
				sort.Slice(ips, func(i, j int) bool {
					return bytes.Compare(ips[i], ips[j]) == -1
				})
				clientC <- Client{
					MAC:      mac,
					IPs:      ips,
					AP:       ap,
					Hostname: ci.Hostname,
					Vendor:   ci.VendorClass,
				}
			}
		}(mac, ap)
	}

	clients := make([]Client, len(connectedClients))
	for i := 0; i < len(connectedClients); i++ {
		clients[i] = <-clientC
	}
	sort.Slice(clients, func(i, j int) bool {
		return bytes.Compare(clients[i].MAC, clients[j].MAC) == -1
	})

	return &Page{Clients: clients}, nil
}

func (d DashServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/static/") {
		d.static.ServeHTTP(w, r)
	} else if r.URL.Path == "/" {
		p, err := d.render()
		if err != nil {
			glog.Errorf("error rendering dashboard: %v", err)
			http.Error(w, "internal server error", 500)
			return
		}
		err = d.index.Execute(w, &p)
		if err != nil {
			glog.Errorf("error applying dashboard template: %v", err)
			http.Error(w, "internal server error", 500)
		}
	} else {
		http.NotFound(w, r)
	}
}

func (d *DashServer) shutdown(ctx context.Context) error {
	err := d.h.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (d *DashServer) shutdownOnSignal() {
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	var socketRename map[string]string
	if *friendlySocketNames != "" {
		if err := json.Unmarshal([]byte(*friendlySocketNames), &socketRename); err != nil {
			glog.Exitf("-hostapd.friendly_socket_names must be JSON string to string map: %v", err)
		}
	}

	static := http.StripPrefix("/static", http.FileServer(http.Dir("./static")))

	index := template.New("root").New("index.html").Funcs(template.FuncMap{
		"join": func(ips []net.IP, sep string) string {
			s := make([]string, len(ips))
			for i, ip := range ips {
				s[i] = ip.String()
			}
			return strings.Join(s, sep)
		},
	})
	if _, err := index.ParseFiles("index.html"); err != nil {
		glog.Exitf("error parsing index.html template: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hostapd, err := NewHostapdControl(ctx)
	if err != nil {
		glog.Exitf("error connecting to hostapd control: %v", err)
	}
	defer hostapd.Close()

	dhcp, err := NewDHCPStore()
	if err != nil {
		glog.Exitf("error connecting to dhcp store: %v", err)
	}
	defer func() {
		if err := dhcp.Close(); err != nil {
			glog.Errorf("error closing dhcp store: %v", err)
		}
	}()

	d := &DashServer{
		index:        index,
		static:       static,
		hostapd:      hostapd,
		dhcp:         dhcp,
		socketRename: socketRename,
	}
	h := &http.Server{
		Addr:           ":8080",
		Handler:        d,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	d.h = h

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := d.shutdown(ctx)
		if err != nil {
			panic(err)
		}
	}()
	glog.Infof("starting dashboard on %v", h.Addr)
	err = h.ListenAndServe()
	switch err {
	case http.ErrServerClosed:
		// Pass
	default:
		glog.Fatal(err)
	}
}
