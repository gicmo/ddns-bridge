// curl -u username:password "http://localhost:5000/post?hostname=test.xatom.net"
package main

import (
	"encoding/base64"
	"fmt"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/fcgi"
	"net/url"
	"strings"
)

var (
	bind    = flag.String("bind", "", "bind address (empty: fcgi)")
	verify  = flag.String("verify", "", "file to read a verification key from")
	debug   = flag.Bool("debug", false, "log some stuff")
)

func init() {
	flag.Parse()
}

func verifyRequest (path, key string) bool {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		println("could not read verification file")
		return false
	}

	bad := " \n"
	return strings.Trim(string(data), bad) == strings.Trim(key, bad)
}

func handler(w http.ResponseWriter, req *http.Request) {

	if debug != nil && *debug {
		log.Printf("req: %s", req.URL.String())
	}

//	rd, _ := httputil.DumpRequest(req, true)
//	fmt.Fprintf(w, string(rd))

	// FROM
	// GET /nic/update?hostname=yourhostname&myip=ipaddress&wildcard=NOCHG&mx=NOCHG&backmx=NOCHG HTTP/1.0
	// Host: members.dyndns.org
	// Authorization: Basic base-64-authorization
	// User-Agent: Company - Device - Version Number

	// TO
	//https://dyndns.regfish.de/?fqdn=<domain>&thisipv4=1&forcehost=1&token=<username>

	var hostname string
	var token string
	var ip string

	//extract auth
	auth := strings.SplitN(req.Header.Get("Authorization"), " ", 2)

        if len(auth) != 2 || auth[0] != "Basic" {
            http.Error(w, "invalid credentials 1", http.StatusUnauthorized)
            return
        }

        data, _ := base64.StdEncoding.DecodeString(auth[1])
        tuple := strings.SplitN(string(data), ":", 2)

        if len(tuple) != 2 {
            http.Error(w, "invalid credentials 2", http.StatusUnauthorized)
            return
        }
	token = tuple[1]

	if verify != nil && len(*verify) != 0 {
		ok := verifyRequest (*verify, tuple[0])
		if !ok {
			http.Error(w, "invalid credentials 3", http.StatusUnauthorized)
			return
		}
	}

	//extract parameters
	keys, ok := req.URL.Query()["hostname"]

	if !ok || len(keys[0]) < 1 {
		http.Error(w, ":(", http.StatusBadRequest)
		return
	}
	hostname = keys[0]

	keys, ok = req.URL.Query()["myip"]

	if !ok || len(keys[0]) < 1 {
		ip = req.Header.Get("X-Forwarded-For")
	} else {
		ip = keys[0]
	}

	r, err := http.NewRequest("GET", "https://dyndns.regfish.de/", nil)
	if err != nil {
		http.Error(w, ":(", http.StatusInternalServerError)
		log.Print(err)
	}

	q := url.Values{}
	q.Add("fqdn", hostname)
	q.Add("thisipv4","0") // we are proxying, it is not our IPv4
	q.Add("ipv4", ip)
	q.Add("token", token)
	r.URL.RawQuery = q.Encode()

	log.Printf("  sending: %s", r.URL.String())

	client := &http.Client{}
	rs, err := client.Do(r)
	if err != nil {
		http.Error(w, ":(", http.StatusInternalServerError)
		log.Print(err)
	}

	defer rs.Body.Close()

	bodyBytes, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		http.Error(w, ":(", http.StatusInternalServerError)
		log.Print(err)
	}

	bodyString := string(bodyBytes)
	log.Printf("  got: %s", bodyString)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, bodyString)
}

func main() {
	var err error

	h := http.HandlerFunc(handler)

	if bind == nil || len(*bind) == 0 {
		err = fcgi.Serve(nil, h)
	} else {
		err = http.ListenAndServe(*bind, h)
	}

	if err != nil {
		log.Fatalln(err)
	}
}
