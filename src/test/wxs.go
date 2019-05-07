package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"utils"
)

var REALBODY string = ""
var wxsconfig utils.Config

func handleCheckFunc(w http.ResponseWriter, req *http.Request) {
	msg := &utils.MSG{}
	msg.MsgSignature = req.URL.Query().Get("msg_signature")
	msg.Timestamp = req.URL.Query().Get("timestamp")
	msg.Nonce = req.URL.Query().Get("nonce")
	msg.EchoStr = req.URL.Query().Get("echostr")
	if msg.Verify(wxsconfig.TOKEN) && req.Method == "GET" {
		w.Write(msg.DecryptStr(msg.EchoStr, wxsconfig.AESKEY))
	} else if req.Method == "POST" {
		body, _ := ioutil.ReadAll(req.Body)
		err := xml.Unmarshal(body, &msg)
		msg.CheckErr(err)
		if msg.Verify(wxsconfig.TOKEN) {
			REALBODY = string(body[:])
		}
		defer req.Body.Close()
	} else {
		w.Write([]byte(wxsconfig.FakeBody))
	}
}
func handleAllFunc(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/", 301)
}

func handleRootFunc(w http.ResponseWriter, req *http.Request) {
	msg := &utils.MSG{}
	msg.MsgSignature = req.URL.Query().Get("msg_signature")
	msg.Timestamp = req.URL.Query().Get("timestamp")
	msg.Nonce = req.URL.Query().Get("nonce")
	msg.EchoStr = req.URL.Query().Get("echostr")
	msg.Encrypt = ""
	if req.Method == "GET" && msg.Verify(wxsconfig.TOKEN) && (string(msg.DecryptStr(msg.EchoStr, wxsconfig.AESKEY)) == wxsconfig.SecretWord) && (len(REALBODY) > 0) {
		w.Write([]byte(REALBODY))
		REALBODY = ""
	} else {
		w.Write([]byte(wxsconfig.FakeBody))
	}
}

func addDefaultHeaders(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", wxsconfig.HeaderServer)
		fn(w, r)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s config.json\n", os.Args[0])
		return
	}
	file_body, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	json.Unmarshal(file_body, &wxsconfig)

	mux := http.NewServeMux()
	mux.HandleFunc("/", addDefaultHeaders(handleRootFunc))
	mux.HandleFunc(wxsconfig.CheckURL, addDefaultHeaders(handleCheckFunc))
	mux.HandleFunc("/*", addDefaultHeaders(handleAllFunc))

	addr := fmt.Sprintf(":%d", wxsconfig.Port)
	if wxsconfig.SSL {
		cfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
		srv := &http.Server{
			Addr:         addr,
			Handler:      mux,
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}
		log.Fatal(srv.ListenAndServeTLS(wxsconfig.CertFile, wxsconfig.KeyFile))
	} else {
		server := &http.Server{
			Addr:    addr,
			Handler: mux,
		}
		server.ListenAndServe()
	}

}
