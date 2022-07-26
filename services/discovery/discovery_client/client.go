// Package discovery_client allows for communication with the Discovery service.
// TODO: Migrate away from net/http; Investigate the usage of fasthttp.

package discovery_client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"gitlab.com/george/shoya-go/models"
)

var NotFoundErr = errors.New("discovery: not found")
var ServerErr = errors.New("discovery: service faced error, check logs")

type Discovery struct {
	c      *http.Client
	Url    string
	ApiKey string
}

func NewDiscovery(url, apiKey string) *Discovery {
	return &Discovery{
		c:      &http.Client{},
		Url:    url,
		ApiKey: apiKey,
	}
}

// GetInstance retrieves live information about an instance.
func (d *Discovery) GetInstance(instance string) *models.WorldInstance {
	b, err := d.doRequest(http.MethodGet, fmt.Sprintf("%s/%s", d.Url, instance))
	if err != nil {
		return nil
	}

	var i = &models.WorldInstance{}
	err = json.Unmarshal(b, i)
	if err != nil {
		return nil
	}

	return i
}

func (d *Discovery) GetInstanceByShortName(shortName string) *models.WorldInstance {
	b, err := d.doRequest(http.MethodGet, fmt.Sprintf("%s/s/%s", d.Url, shortName))
	if err != nil {
		return nil
	}

	var i = &models.WorldInstance{}
	err = json.Unmarshal(b, i)
	if err != nil {
		return nil
	}

	return i
}

// GetInstancesForWorld retrieves a list of instances for a specified world id.
func (d *Discovery) GetInstancesForWorld(world string) []*models.WorldInstance {
	var i []*models.WorldInstance

	b, err := d.doRequest(http.MethodGet, fmt.Sprintf("%s/world/%s", d.Url, world))
	if err != nil {
		if err == NotFoundErr {
			return nil
		}
		return nil
	}

	err = json.Unmarshal(b, &i)
	if err != nil {
		return nil
	}

	return i
}

// RegisterInstance registers an instance in Redis.
func (d *Discovery) RegisterInstance(instance string, capacity int) *models.WorldInstance {
	var i *models.WorldInstance
	b, err := d.doRequest(http.MethodPost, fmt.Sprintf("%s/register/%s?capacity=%d", d.Url, instance, capacity))
	if err != nil {
		if err == NotFoundErr {
			return nil
		}
		return nil
	}

	err = json.Unmarshal(b, &i)
	if err != nil {
		return nil
	}

	return i
}

// PingInstance updates the lastPing in Redis.
func (d *Discovery) PingInstance(instance string) {
	d.c.Post(fmt.Sprintf("%s/ping/%s?apiKey=%s", d.Url, instance, d.ApiKey), "application/json", nil)
}

// UnregisterInstance removes an instance from Redis.
func (d *Discovery) UnregisterInstance(instance string) {
	d.c.Post(fmt.Sprintf("%s/unregister/%s?apiKey=%s", d.Url, instance, d.ApiKey), "application/json", nil)
}

// FindPlayer finds what instance(s) a player is in.
func (d *Discovery) FindPlayer(player string) []*models.WorldInstance {
	var i []*models.WorldInstance

	b, err := d.doRequest(http.MethodGet, fmt.Sprintf("%s/player/%s", d.Url, player))
	err = json.Unmarshal(b, &i)
	if err != nil {
		return nil
	}

	return i
}

// AddPlayerToInstance adds a player to an instance in Redis.
func (d *Discovery) AddPlayerToInstance(player, instance string) {
	d.doRequest(http.MethodPut, fmt.Sprintf("%s/player/%s/%s", d.Url, instance, player))
}

// RemovePlayerFromInstance removes a player from an instance in Redis.
func (d *Discovery) RemovePlayerFromInstance(player, instance string) {
	d.doRequest(http.MethodDelete, fmt.Sprintf("%s/player/%s/%s", d.Url, instance, player))
}

func (d *Discovery) doRequest(method, url string) ([]byte, error) {
	r, _ := http.NewRequest(method, url, nil)
	r.Header.Add("Authorization", d.ApiKey)

	do, err := d.c.Do(r)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()

	if do.StatusCode != 200 {
		if do.StatusCode == 404 {
			return nil, NotFoundErr
		}
		return nil, ServerErr
	}

	b, err := io.ReadAll(do.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
