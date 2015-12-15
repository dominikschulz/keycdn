package keycdn

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const BaseURL = "https://api.keycdn.com"

type Client struct {
	apikey string
	Base   string
	http   *http.Client
}

func New(key string) Client {
	return Client{
		apikey: key,
		Base:   BaseURL,
	}
}

type response struct {
	Status      string `json:"status"`
	Description string `json:"description"`
}

type Zone struct {
	ID                      uint64
	Name                    string
	Status                  string
	Type                    string
	ForceDownload           bool
	CORS                    bool
	Gzip                    bool
	Expire                  int
	HTTP2                   bool
	SecureToken             bool
	SecureTokenKey          string
	SSLCert                 string
	CustomSSLKey            string
	CunstomSSLCert          string
	ForceSSL                bool
	OriginURL               string
	CacheMaxExpire          int
	CacheIgnoreCacheControl bool
	CacheIgnoreQueryString  bool
	CacheStripCookies       bool
	CachePullKey            string
	CacheCanonical          bool
	CacheRobots             bool
}

type zonesResp struct {
	response
	Data map[string][]zoneResp
}

type zoneResp map[string]string

func (z zoneResp) ToZone() Zone {
	zone := Zone{}
	if idStr, found := z["id"]; found {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err == nil {
			zone.ID = id
		}
	}
	if name, found := z["name"]; found {
		zone.Name = name
	}
	// TODO fill out other fields as well
	return zone
}

type stateStatResponse struct {
	response
	Data map[string][]stateAmountResp `json:"data"`
}

type stateAmountResp map[string]string

func (s stateAmountResp) Get(key string) uint64 {
	if v, found := s[key]; found {
		iv, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return uint64(iv)
	}
	return 0
}

type trafficAmountResp struct {
	Amount    string `json:"amount"`
	Timestamp string `json:"timestamp"`
}

func (t trafficAmountResp) Count() uint64 {
	iv, err := strconv.Atoi(t.Amount)
	if err != nil {
		return 0
	}
	return uint64(iv)
}

func (t trafficAmountResp) Time() time.Time {
	iv, err := strconv.Atoi(t.Timestamp)
	if err != nil {
		return time.Now()
	}
	return time.Unix(int64(iv), 0)
}

type trafficResponse struct {
	response
	Data map[string][]trafficAmountResp `json:"data"`
}

func (c Client) Zones() (map[uint64]Zone, error) {
	zones := make(map[uint64]Zone, 2)
	b, err := c.get("/zones.json", map[string]string{})
	if err != nil {
		return zones, err
	}
	var zr zonesResp
	err = json.Unmarshal(b, &zr)
	if err != nil {
		return zones, err
	}
	if _, found := zr.Data["zones"]; !found {
		return zones, fmt.Errorf("zones not found in data")
	}
	for _, z := range zr.Data["zones"] {
		zone := z.ToZone()
		zones[zone.ID] = zone
	}
	return zones, nil
}

func (c Client) Traffic(zoneID uint64, from, to time.Time) (uint64, error) {
	args := make(map[string]string, 4)
	args["zone_id"] = strconv.FormatUint(zoneID, 10)
	args["start"] = strconv.Itoa(int(from.Unix()))
	args["end"] = strconv.Itoa(int(to.Unix()))
	args["interval"] = "hour"
	b, err := c.get("/reports/traffic.json", args)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("Body: %s\n", b)
	var tr trafficResponse
	err = json.Unmarshal(b, &tr)
	if err != nil {
		return 0, err
	}
	if _, found := tr.Data["stats"]; !found {
		return 0, fmt.Errorf("stats not found in data")
	}
	var sum uint64
	for _, a := range tr.Data["stats"] {
		//fmt.Printf("TS: %s - Amount: %d\n", a.Time().Format(time.RFC3339), a.Count()/1024/1024)
		sum += a.Count()
	}
	return sum, nil
}

func (c Client) Status(zoneID uint64, from, to time.Time) (map[string]uint64, error) {
	ret := make(map[string]uint64, 4)
	args := make(map[string]string, 4)
	args["zone_id"] = strconv.FormatUint(zoneID, 10)
	args["start"] = strconv.Itoa(int(from.Unix()))
	args["end"] = strconv.Itoa(int(to.Unix()))
	args["interval"] = "hour"
	b, err := c.get("/reports/statestats.json", args)
	if err != nil {
		return ret, err
	}
	var ssr stateStatResponse
	err = json.Unmarshal(b, &ssr)
	if err != nil {
		return ret, err
	}
	if _, found := ssr.Data["stats"]; !found {
		return ret, fmt.Errorf("stats not found in data")
	}
	for _, a := range ssr.Data["stats"] {
		for _, k := range []string{"totalcachehit", "totalcachemiss", "totalsuccess", "totalerror"} {
			ret[k] += a.Get(k)
		}
	}
	return ret, nil
}

func (c Client) get(file string, args map[string]string) ([]byte, error) {
	vs := url.Values{}
	for k, v := range args {
		vs.Set(k, v)
	}
	url := c.Base + file + "?" + vs.Encode()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}
	req.SetBasicAuth(c.apikey, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
