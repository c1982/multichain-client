package multichain

import (
	"fmt"
	"time"
	"errors"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"
	//
	"golang.org/x/net/context"
	"google.golang.org/appengine/urlfetch"
	//
	"github.com/dghubble/sling"
)

const (
	CONST_ID = "multichain-client"
)

type Response map[string]interface{}

func (r Response) Result() interface{} {
	return r["result"]
}

type Client struct {
	Domain string
	chain string
	httpClient *http.Client
	port string
	endpoints []string
	credentials string
	debug bool
}

func NewClient(chain, host, port, username, password string) *Client {

	credentials := username + ":" + password

	return &Client{
		Domain: host,
		chain: chain,
		httpClient: &http.Client{},
		port: port,
		endpoints: []string{fmt.Sprintf("http://%s:%s", host, port)},
		credentials: base64.StdEncoding.EncodeToString([]byte(credentials)),
	}
}

func (client *Client) DebugMode() {
	client.debug = true
}


func (client *Client) Urlfetch(ctx context.Context, duration time.Duration) {

	ctx, _ = context.WithDeadline(ctx, time.Now().Add(duration))

	client.httpClient = urlfetch.Client(ctx)
}

func (client *Client) msg(params []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "1.0",
		"id": CONST_ID,
		"params": params,
	}
}

func (client *Client) NodeMsg(method string, params []interface{}) map[string]interface{} {

	msg := client.msg(params)
	msg["method"] = fmt.Sprintf("%s", method)

	if client.debug {
		fmt.Println(msg)
	}

	return msg
}

func (client *Client) ChainMsg(method string, params []interface{}) map[string]interface{} {

	msg := client.msg(params)
	msg["method"] = fmt.Sprintf("%s %s", client.chain, method)

	if client.debug {
		fmt.Println(msg)
	}

	return msg
}

// Creates a new temporary config for calling an RPC method on the specified node
func (client *Client) ViaNodes(hosts []int) *Client {

	c := *client
	c.endpoints = []string{}

	for _, host := range hosts {

		c.endpoints = append(c.endpoints, fmt.Sprintf("http://%v.%s:%s", host, client.Domain, client.port))

	}

	return &c
}

// Creates a new temporary config for calling an RPC method on the specified node
func (client *Client) ViaNode(subdomain string) *Client {

	endpoint := fmt.Sprintf("http://%s.%s:%s", subdomain, client.Domain, client.port)

	c := *client
	c.endpoints = []string{
		endpoint,
		endpoint,
	}

	return &c
}


func (client *Client) post(msg interface{}) (Response, error) {

	if client.debug {
		fmt.Println("DEBUG MODE ON...")
		fmt.Println(client)
		b, _ := json.Marshal(msg)
		fmt.Println(string(b))
	}

	for i, endpoint := range client.endpoints {

		request, err := sling.New().Post(endpoint).BodyJSON(msg).Request()
		if err != nil {
			return nil, err
		}

		request.Header.Add("Authorization", "Basic " + client.credentials)

		resp, err := client.httpClient.Do(request)
		if err != nil {
			if (i + 1) == len(client.endpoints) {
				return nil, err
			}
			continue
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if client.debug {
			fmt.Println(string(b))
		}

		obj := make(Response)

		err = json.Unmarshal(b, &obj)
		if err != nil {
			return nil, err
		}

		if obj["error"] != nil {
			e := obj["error"].(map[string]interface{})
			var s string
			m, ok := msg.(map[string]interface{})
			if ok {
				s = fmt.Sprintf("multichaind - '%s': %s", m["method"], e["message"].(string))
			} else {
				s = fmt.Sprintf("multichaind - %s", e["message"].(string))
			}
			if (i + 1) == len(client.endpoints) {
				return nil, errors.New(s)
			}
			continue
		}

		if resp.StatusCode != 200 {
			if (i + 1) == len(client.endpoints) {
				return nil, err
			}
			continue
		}

		return obj, nil
	}

	return nil, errors.New("PROBABLY NO ENDPOINTS PASSED TO THE REQUEST DISPATCHER")
}
