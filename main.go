package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

const baseURL string = "[USM URL HERE]"

var searchAssetsBody = []byte(`{"define":{"a":{"type":"Asset"},"g":{"type":"AssetGroup","join":"a","relationship":"AssetMemberOfAssetGroup","fromLeft":true},"s":{"type":"Service","join":"a","relationship":"AssetHasService","fromLeft":true},"c":{"type":"CPEItem","join":"a","relationship":"AssetHasCPEItem","fromLeft":true},"p":{"type":"Plugin","join":"a","relationship":"AssetHasPlugin","fromLeft":true}},"where":[{"and":{"==":{"a.knownAsset":"true"}}}],"return":{"assets":{"object":"a","page":{"start":0,"count":250},"inject":{"AssetHasNetworkInterface":{"relationship":"AssetHasNetworkInterface","fromLeft":true,"inject":{"NetworkInterfaceHasHostname":{"relationship":"NetworkInterfaceHasHostname","fromLeft":true}}},"AssetHasCredentials":{"relationship":"AssetHasCredentials","fromLeft":true},"AssetHasAgent":{"relationship":"AssetHasAgent","fromLeft":true}},"sort":["a.dateUpdated desc"]},"agg_operatingSystem":{"aggregation":"a.operatingSystem","sort":["count desc","value asc"],"count":50},"agg_deviceType":{"aggregation":"a.deviceType","sort":["count desc","value asc"],"count":50},"agg_assetOriginType":{"aggregation":"a.assetOriginType","sort":["count desc","value asc"],"count":50},"agg_AssetMemberOfAssetGroup":{"aggregation":"g.id","sort":["count desc","value asc"]},"agg_assetService":{"aggregation":"s.data","sort":["count desc","value asc"],"count":50},"agg_assetSoftware":{"aggregation":"c.name","sort":["count desc","value asc"],"count":50},"agg_assetPlugin":{"aggregation":"p.name","sort":["count desc","value asc"],"count":50},"agg_assetOriginUUID":{"aggregation":"a.assetOriginUUID","sort":["count desc","value asc"],"count":50}}}`)

type State struct {
	Client      *Client
	UpdateAssets bool
	Verbose      bool
	Domain		 string
}

type AssetResult struct {
	Json  []byte
	Asset Asset
}

type NetworkInterfaceHostname struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type AssetNetworkInterface struct {
	IpAddress             string                     `json:"ipAddress"`
	NetworkInterfaceHosts []NetworkInterfaceHostname `json:"NetworkInterfaceHasHostname"`
}

type Client struct {
	UserAgent  string
	httpClient *http.Client
	Cookie     string
	XSRFToken  string
}

type Asset struct {
	Id                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	AssetNetworkInterfaces []AssetNetworkInterface `json:"AssetHasNetworkInterface"`
}

type Assets struct {
	Assets []Asset     `json:"results"`
	Total  json.Number `json:"total"`
}

type QueryResult struct {
	Assets Assets `json:"assets"`
}

func ParseCmdLine() *State {
	//valid := true

	s := State{}

	c := &Client{
		httpClient: http.DefaultClient,
	}

	flag.StringVar(&c.Cookie, "c", "", "Cookies to use for the request")
	flag.StringVar(&c.UserAgent, "u", "", "User Agent string")
	flag.StringVar(&c.XSRFToken, "x", "", "XSRF Token")
	flag.StringVar(&s.Domain, "d", "", "Base domain to be used in FQDN (i.e. -d mycompany.com")

	flag.BoolVar(&s.Verbose, "v", false, "Verbose output")

	cookieJar, _ := cookiejar.New(nil)
	c.httpClient.Jar = cookieJar

	flag.Parse()
	s.Client = c

	return &s
}

func (c *Client) GetAssetDetailsWithChannel(asset Asset, ch chan<- *AssetResult) {
	path := "/api/1.0/assets/" + asset.Id
	req, err := c.newRequest("GET", path, nil)
	if err != nil {
		log.Fatal(err)
	}

	result, err := c.do(req)
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	r := AssetResult{result, asset}

	ch <- &r
}

func (c *Client) UpdateAssetDetailsWithChannel(asset Asset, body []byte, ch chan<- *Asset) {
	path := "/api/1.0/assets/" + asset.Id
	req, err := c.newRequest("PUT", path, body)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.do(req)
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	ch <- &asset
}

func (c *Client) ListAssets() (*QueryResult, error) {
	req, err := c.newRequest("POST", "/api/1.0/search/aql", searchAssetsBody)
	if err != nil {
		return nil, err
	}

	result, err := c.do(req)

	if err != nil {
		return nil, err
	}
	var assetsList QueryResult
	err = json.Unmarshal(result, &assetsList)

	if err != nil {
		return nil, err
	}
	return &assetsList, err
}

func (c *Client) newRequest(method, path string, body []byte) (*http.Request, error) {
	rel := &url.URL{Path: path}
	b, err := url.Parse(baseURL)
	u := b.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, u.String(), buf)
	ck := http.Cookie{
		Name:  "JSESSIONID",
		Value: c.Cookie,
	}

	req.AddCookie(&ck)

	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-XSRF-TOKEN", c.XSRFToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), `"statusCode":401`) {
		log.Fatal("ERROR: Your secretKey and accessKey (credentials) are invalid. ")
	}

	if 200 != resp.StatusCode {
		return nil, fmt.Errorf("%s", body)
	}
	return body, err
}

func Process(s *State) {
	fmt.Print("\n\n==========================================\n")
	r, err := s.Client.ListAssets()
	if err != nil {
		fmt.Println(err)
	}
	assetsList := r.Assets.Assets
	ch := make(chan *AssetResult)

	var orphaned []Asset
	for _, v := range assetsList {
		hasFqdn := false
		for _, ni := range v.AssetNetworkInterfaces {
			for _, host := range ni.NetworkInterfaceHosts {
				if host.Name != "" {
					hasFqdn = true
				}
			}
		}
		if !hasFqdn {
			orphaned = append(orphaned, v)
			go s.Client.GetAssetDetailsWithChannel(v, ch)
		}
	}

	if s.UpdateAssets{
		uch := make(chan *Asset)
		fmt.Println("Assets without FQDN:")
		fmt.Println("==========================================")
		for _, _ = range orphaned {
			assetDetails := <-ch
			idx := strings.Index(string(assetDetails.Json), "NetworkInterfaceHasHostname\":[{\"name\":\"\",")
			if idx != -1 {
				fmt.Println("Asset is missing FQDN ------>  " + assetDetails.Asset.Name + " ")
				changed := strings.Replace(string(assetDetails.Json), "\"NetworkInterfaceHasHostname\":[{\"name\":\"\"", "\"NetworkInterfaceHasHostname\":[{\"name\":\""+assetDetails.Asset.Name+"." + s.Domain + "\"", -1)
				go s.Client.UpdateAssetDetailsWithChannel(assetDetails.Asset, []byte(changed), uch)
			}
		}

		fmt.Printf("\n TOTAL ASSETS WITHOUT FQDN = %d\n", len(orphaned))
		fmt.Println("==========================================")
		fmt.Printf("\nUpdated assets with missing FQDN:\n")
		fmt.Println("==========================================")
		for _, _ = range orphaned {
			updatedAsset := <-uch
			fmt.Println("Succesfully updated " + updatedAsset.Name)
		}
	}

	os.Exit(0)
}

func main() {
	state := ParseCmdLine()
	if state != nil {
		Process(state)
	}
}
