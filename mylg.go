// myLG is command line looking glass that written with Go language
// it tries from its own icmp and external looking glasses tools
package main

import (
	"errors"
	"github.com/briandowns/spinner"
	"github.com/mehrdadrad/mylg/cli"
	"github.com/mehrdadrad/mylg/dns"
	"github.com/mehrdadrad/mylg/icmp"
	"github.com/mehrdadrad/mylg/lg"
	"github.com/mehrdadrad/mylg/ripe"
	"net"
	"regexp"
	"strings"
	"time"
)

type Provider interface {
	Set(host, version string)
	GetDefaultNode() string
	GetNodes() []string
	ChangeNode(node string)
	Ping() (string, error)
}

var (
	providers = map[string]Provider{"telia": new(lg.Telia), "level3": new(lg.Level3), "cogent": new(lg.Cogent)}
	pNames    = providerNames()
)

func providerNames() []string {
	pNames := []string{}
	for p := range providers {
		pNames = append(pNames, p)
	}
	return pNames
}

func validateProvider(p string) (string, error) {
	pNames := []string{}
	match, _ := regexp.MatchString("("+strings.Join(pNames, "|")+")", p)
	p = strings.ToLower(p)
	if match {
		return p, nil
	} else {
		return "", errors.New("provider not support")
	}
}

func main() {
	var (
		err     error
		request string
		loop    bool   = true
		cPName  string = "local"
	)

	rep := make(chan string, 1)
	req := make(chan string, 1)
	nxt := make(chan struct{}, 1)

	c := cli.Init("local")
	go c.Run(req, nxt)

	r, _ := regexp.Compile(`(ping|lg|dns|asn|connect|node|local|mode|help|exit|quit)\s{0,1}(.*)`)
	s := spinner.New(spinner.CharSets[26], 220*time.Millisecond)

	for loop {
		select {
		case request, loop = <-req:
			if !loop {
				break
			}
			subReq := r.FindStringSubmatch(request)
			if len(subReq) == 0 {
				println("syntax error")
				c.Next()
				continue
			}
			prompt := c.GetPrompt()
			cmd := strings.TrimSpace(subReq[1])
			args := strings.TrimSpace(subReq[2])
			switch {
			case cmd == "ping" && cPName == "local":
				p := icmp.NewPing()
				ra, err := net.ResolveIPAddr("ip", args)
				if err != nil {
					println("cannot resolve", args, ": Unknown host")
					c.Next()
					continue
				}
				p.IP(ra.String())
				for n := 0; n < 4; n++ {
					p.Ping(rep)
					println(<-rep)
				}
				c.Next()
			case cmd == "ping":
				s.Prefix = "please wait "
				s.Start()
				providers[cPName].Set(args, "ipv4")
				m, _ := providers[cPName].Ping()
				s.Stop()
				println(m)
				c.Next()
			case cmd == "node":
				if _, ok := providers[cPName]; ok {
					providers[cPName].ChangeNode(args)
					c.SetPrompt(cPName + "/" + args)
				} else {
					println("it doesn't support")
				}
				c.Next()
			case cmd == "local":
				cPName = "local"
				c.SetPrompt(cPName)
				c.Next()
			case cmd == "lg":
				c.SetPrompt("lg")
				c.UpdateCompleter("connect", pNames)
				c.Next()
			case cmd == "connect":
				if strings.HasPrefix(prompt, "dns") {
					println("todo")
				} else {
					var pName string
					if pName, err = validateProvider(args); err != nil {
						println("provider not available")
						c.Next()
						continue
					}
					cPName = pName
					if _, ok := providers[cPName]; ok {
						c.SetPrompt(cPName + "/" + providers[cPName].GetDefaultNode())
						go func() {
							c.UpdateCompleter("node", providers[cPName].GetNodes())
						}()
					} else {
						println("it doesn't support")
					}
				}
				c.Next()
			case cmd == "dns":
				d := dns.NewRequest()
				go d.Init(c)
				c.Next()
			case cmd == "asn":
				asn := ripe.ASN{Number: args}
				asn.GetData()
				asn.PrettyPrint()
				c.Next()
			case cmd == "mode":
				if args == "vim" {
					c.SetVim()
				} else if args == "emacs" {
					c.SetEmacs()
				} else {
					println("the request mode doesn't support")
				}
				c.Next()
			case cmd == "help":
				c.Help()
				c.Next()
			case cmd == "exit", cmd == "quit":
				c.Close(nxt)
				close(req)
				// todo
			}
		}
	}
}