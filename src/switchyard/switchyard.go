package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	//	"os"
	"strings"
)

var fwd_port = flag.Int("port", 8888, "Port to forward virtualhosts")
var cfg_port = flag.Int("cfg_port", 8889, "Port to configure switchyard")

type ForwardSpec struct {
	Hostname string
	Target   string
}

type RequestHandler struct {
	Transport    *http.Transport
	Forwards     []*ForwardSpec
	AddForwarded bool
}

func Copy(dest *bufio.ReadWriter, src *bufio.ReadWriter) {
	buf := make([]byte, 40*1024)
	for {
		n, err := src.Read(buf)
		if err != nil && err != io.EOF {
			log.Printf("Read failed: %v", err)
			return
		}
		if n == 0 {
			return
		}
		dest.Write(buf[0:n])
		dest.Flush()
	}
}

func CopyBidir(conn1 io.ReadWriteCloser, rw1 *bufio.ReadWriter, conn2 io.ReadWriteCloser, rw2 *bufio.ReadWriter) {
	finished := make(chan bool)

	go func() {
		Copy(rw2, rw1)
		conn2.Close()
		finished <- true
	}()
	go func() {
		Copy(rw1, rw2)
		conn1.Close()
		finished <- true
	}()

	<-finished
	<-finished
}

func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("incoming request: %#v\n", *r)
	r.RequestURI = ""
	r.URL.Scheme = "http"

	if h.AddForwarded {
		remote_addr := r.RemoteAddr
		idx := strings.LastIndex(remote_addr, ":")
		if idx != -1 {
			remote_addr = remote_addr[0:idx]
			if remote_addr[0] == '[' && remote_addr[len(remote_addr)-1] == ']' {
				remote_addr = remote_addr[1 : len(remote_addr)-1]
			}
		}
		r.Header.Add("X-Forwarded-For", remote_addr)
	}

	has_a_host := false
	var fwd *ForwardSpec
	for _, fwd = range h.Forwards {
		if fwd.Hostname == r.Host {
			has_a_host = true
			break
		}
	}
	if !has_a_host {
		http.Error(w, "no suitable backend found for request", http.StatusServiceUnavailable)
		return
	}
	r.URL.Host = fwd.Target
	conn_hdr := ""
	conn_hdrs := r.Header["Connection"]
	//log.Printf("Connection headers: %v", conn_hdrs)
	if len(conn_hdrs) > 0 {
		conn_hdr = conn_hdrs[0]
	}

	upgrade_websocket := false
	if strings.ToLower(conn_hdr) == "upgrade" {
		//	log.Printf("got Connection: Upgrade")
		upgrade_hdrs := r.Header["Upgrade"]
		//	log.Printf("Upgrade headers: %v", upgrade_hdrs)
		if len(upgrade_hdrs) > 0 {
			upgrade_websocket = (strings.ToLower(upgrade_hdrs[0]) == "websocket")
		}
	}

	if upgrade_websocket {
		hj, ok := w.(http.Hijacker)

		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}

		conn, bufrw, err := hj.Hijack()
		defer conn.Close()

		conn2, err := net.Dial("tcp", r.URL.Host)
		if err != nil {
			http.Error(w, "couldn't connect to backend server", http.StatusServiceUnavailable)
			return
		}
		defer conn2.Close()

		err = r.Write(conn2)
		if err != nil {
			log.Printf("writing WebSocket request to backend server failed: %v", err)
			return
		}

		CopyBidir(conn, bufrw, conn2, bufio.NewReadWriter(bufio.NewReader(conn2), bufio.NewWriter(conn2)))

	} else {

		resp, err := h.Transport.RoundTrip(r)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Error: %v\n", err)
			return
		}

		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		w.WriteHeader(resp.StatusCode)

		io.Copy(w, resp.Body)
		resp.Body.Close()
	}
}

func AddNew(routes chan *ForwardSpec, handler *RequestHandler) {
	for new_fwd := range routes {
		fmt.Println("Adding ", new_fwd.Hostname)
		handler.Forwards = append(handler.Forwards, new_fwd)
	}
}
func ServeFwd(routes chan *ForwardSpec) {
	forward_list := make([]*ForwardSpec, 0, 20)
	mux := http.NewServeMux()
	var request_handler http.Handler = &RequestHandler{
		Transport: &http.Transport{
			DisableKeepAlives:  false,
			DisableCompression: false},
		Forwards: forward_list}

	mux.Handle("/", request_handler)

	addr := fmt.Sprintf(":%d", *fwd_port)
	fmt.Println(addr)
	srv := &http.Server{Handler: mux, Addr: addr}

	/*if f.HTTPS {*/
	/*if err := srv.ListenAndServeTLS(f.CertFile, f.KeyFile); err != nil {*/
	/*log.Printf("Starting HTTPS frontend %s failed: %v", f.Name, err)*/
	/*}*/
	/*} else {*/
	go AddNew(routes, request_handler.(*RequestHandler))
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("Starting frontend failed: %v", err)
	}

}

// And here we begin the configuration portion!

var templates = template.Must(template.ParseFiles("templates/index.html"))

type RootHandler struct {
	Forwards []*ForwardSpec
	Routes   chan *ForwardSpec
}

var add_templ = `
<tr>
<td> {{.Hostname}} </td>
<td> {{.Target}} </td>
</tr>
`

func (h *RootHandler) HandleAdd(w http.ResponseWriter, req *http.Request) {
	host := req.URL.Query().Get("host")
	target := req.URL.Query().Get("target")
	if host != "" && target != "" {
		h.AddForward(host, target, w)
	}
}

func (h *RootHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/add" {
		h.HandleAdd(w, req)
		return
	}
	err := templates.ExecuteTemplate(w, "index.html", h)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func (h *RootHandler) AddForward(host, target string, w http.ResponseWriter) {
	fwd := &ForwardSpec{Hostname: host, Target: target}
	h.Forwards = append(h.Forwards, fwd)
	h.Routes <- fwd
	if w != nil {
		t := template.New("Add template")
		t, _ = t.Parse(add_templ)
		t.Execute(w, fwd)
	}
}

func ServeCfg(routes chan *ForwardSpec) {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	// TODO(barakmich): Create the roothandler's initial state here
	handler := &RootHandler{
		Forwards: make([]*ForwardSpec, 0, 20),
		Routes:   routes,
	}
	handler.AddForward("switchyard.app.barakmich.com", "10.42.0.2:8889", nil)
	mux.Handle("/", handler)
	addr := fmt.Sprintf(":%d", *cfg_port)
	srv := &http.Server{Handler: mux, Addr: addr}
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("Starting configuration failed: %v", err)
	}
}

func main() {
	done := make(chan bool)
	new_routes := make(chan *ForwardSpec)
	go ServeFwd(new_routes)
	go ServeCfg(new_routes)
	fmt.Println("Starting Switchyard on", *fwd_port, "forward, ", *cfg_port, "config.")
	for i := 0; i < 1; i++ {
		<-done
	}

}
