package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"github.com/alyu/configparser"
	"log"
	"os"
	"flag"
	"crypto/tls"
	"fmt"
	"strings"
	"regexp"
	"os/exec"
	"encoding/xml"
)

var (
	statuses = map[xmpp.Status]string{
		xmpp.StatusUnconnected : "StatusUnconnected",
		xmpp.StatusConnected : "StatusConnected",
		xmpp.StatusConnectedTls : "StatusConnectedTLS",
		xmpp.StatusAuthenticated : "StatusAuthenticated",
		xmpp.StatusBound : "StatusBound",
		xmpp.StatusRunning : "StatusRunning",
		xmpp.StatusShutdown : "StatusShutdown",
		xmpp.StatusError : "StatusError",
	}
)


func usage() {
	fmt.Printf("usage: %s conf.ini\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func process(msg *xmpp.Message, c *xmpp.Client) {
	body := ""
	if len(msg.Body) > 0 {
		body = strings.ToLower(msg.Body[0].Chardata)
	}
	if body == "" {
		return
	}

	pingRe, _ := regexp.Compile(`ping\s+(.*)`)

	if pingRe.MatchString(body) {
		re := pingRe.FindStringSubmatch(body)
		log.Printf("ping %s for %s", re[1], msg.From.Bare())
		out, _ := exec.Command("ping", "-c5", re[1]).CombinedOutput()
		c.Send <- makeReplay(msg, string(out))


	}
}

func makeReplay(msg *xmpp.Message, body string) *xmpp.Message {
	reply := &xmpp.Message{}
	reply.From = msg.To
	reply.To = msg.From
	reply.Id = xmpp.NextId()
	reply.Type = "chat"
	reply.Lang = "en"
	if msg.Thread != nil {
		reply.Thread = &xmpp.Data{XMLName: xml.Name{Local: "thread"}, Chardata: msg.Thread.Chardata}
	}
	reply.Body = []xmpp.Text{{XMLName: xml.Name{Local: "body"}, Chardata: body}}
	return reply
}


func main() {
	flag.Usage = usage
	debug := flag.Bool("d", false, "debug")
	conffile := flag.String("c", "config.ini", "config file")
	flag.Parse()

	if *debug {
		xmpp.Debug = true
	}

	config, err := configparser.Read(*conffile)
	if err != nil {
	        log.Fatal(*conffile, err)
	}
	section, err := config.Section("global")
	if err != nil {
	        log.Fatal(err)
	}

	jid := xmpp.JID(section.ValueOf("jid"))
	pass := section.ValueOf("pass")

	status := make(chan xmpp.Status, 10)
	go func() {
		for s := range status {
			log.Printf("connection status %s", statuses[s])
		}

	}()
	jid = xmpp.JID(fmt.Sprintf("%s/%s", jid.Bare(), "bot"))
	c, err := xmpp.NewClient(&jid, pass, tls.Config{InsecureSkipVerify: true}, nil, xmpp.Presence{}, status)

	if err != nil {
		log.Fatalf("NewClient(%v): %v", jid, err)
	}
	defer c.Close()
	for {
		select {
		case s := <-status:
			log.Printf("connection status %s", statuses[s])
			if s.Fatal() {
				return
			}
		case st, ok := <-c.Recv:
			if !ok {
				return
			}
			if m, ok := st.(*xmpp.Message); ok {
				log.Printf("msg from %s", m.From.Bare())
				process(m, c)
			}
			if p, ok := st.(*xmpp.Presence); ok {
				log.Printf("presence from %s", p.From.Bare())
			}
			if iq, ok := st.(*xmpp.Iq); ok {
				log.Printf("iq from %s", iq.From.Bare())
			}
		}
	}
}
