package he

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/TimothyYe/godns/notify"

	"github.com/TimothyYe/godns"
)

var (
	// HEUrl the API address for he.net
	HEUrl = "https://dyn.dns.he.net/nic/update"
)

// Handler struct
type Handler struct {
	Configuration *godns.Settings
}

// SetConfiguration pass dns settings and store it to handler instance
func (handler *Handler) SetConfiguration(conf *godns.Settings) {
	handler.Configuration = conf
}

// DomainLoop the main logic loop
func (handler *Handler) DomainLoop(domain *godns.Domain, panicChan chan<- godns.Domain) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Recovered in %v: %v\n", err, string(debug.Stack()))
			panicChan <- *domain
		}
	}()

	looping := false
	for {
		if looping {
			// Sleep with interval
			log.Printf("Going to sleep, will start next checking in %d seconds...\r\n", handler.Configuration.Interval)
			time.Sleep(time.Second * time.Duration(handler.Configuration.Interval))
		}
		looping = true

		currentIP, err := godns.GetCurrentIP(handler.Configuration)

		if err != nil {
			log.Println("get_currentIP:", err)
			continue
		}
		log.Println("currentIP is:", currentIP)

		//check against locally cached IP, if no change, skip update

		for _, subDomain := range domain.SubDomains {
			var hostname string

			if subDomain != godns.RootDomain {
				hostname = subDomain + "." + domain.DomainName
			} else {
				hostname = domain.DomainName
			}

			lastIP, err := godns.ResolveDNS(hostname, handler.Configuration.Resolver, handler.Configuration.IPType)
			if err != nil {
				log.Println(err)
				continue
			}

			//check against currently known IP, if no change, skip update
			if currentIP == lastIP {
				log.Printf("IP is the same as cached one. Skip update.\n")
			} else {
				log.Printf("%s.%s - Start to update record IP...\n", subDomain, domain.DomainName)
				handler.UpdateIP(domain.DomainName, subDomain, currentIP)

				// Send notification
				notify.GetNotifyManager(handler.Configuration).Send(fmt.Sprintf("%s.%s", subDomain, domain.DomainName), currentIP)
			}
		}
	}

}

// UpdateIP update subdomain with current IP
func (handler *Handler) UpdateIP(domain, subDomain, currentIP string) {
	values := url.Values{}

	if subDomain != godns.RootDomain {
		values.Add("hostname", fmt.Sprintf("%s.%s", subDomain, domain))
	} else {
		values.Add("hostname", domain)
	}
	values.Add("password", handler.Configuration.Password)
	values.Add("myip", currentIP)

	client := godns.GetHttpClient(handler.Configuration, handler.Configuration.UseProxy)

	req, _ := http.NewRequest("POST", HEUrl, strings.NewReader(values.Encode()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Request error...")
		log.Println("Err:", err.Error())
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK {
			log.Println("Update IP success:", string(body))
		} else {
			log.Println("Update IP failed:", string(body))
		}
	}
}
