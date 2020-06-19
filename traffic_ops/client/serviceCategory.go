/*

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package client

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/apache/trafficcontrol/lib/go-tc"
)

const (
	API_SERVICE_CATEGORIES = apiBase + "/service-categories"
)

// CreateServiceCategory performs a post to create a service category.
func (to *Session) CreateServiceCategory(serviceCategory tc.ServiceCategory) (tc.Alerts, ReqInf, error) {

	var remoteAddr net.Addr
	reqBody, err := json.Marshal(serviceCategory)
	reqInf := ReqInf{CacheHitStatus: CacheHitStatusMiss, RemoteAddr: remoteAddr}
	if err != nil {
		return tc.Alerts{}, reqInf, err
	}
	resp, remoteAddr, err := to.request(http.MethodPost, API_SERVICE_CATEGORIES, reqBody)
	if err != nil {
		return tc.Alerts{}, reqInf, err
	}
	defer resp.Body.Close()
	var alerts tc.Alerts
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return tc.Alerts{}, reqInf, err
	}
	return alerts, reqInf, nil
}

// UpdateServiceCategoryByID updates a service category by its unique numeric ID.
func (to *Session) UpdateServiceCategoryByID(id int, serviceCategory tc.ServiceCategory) (tc.Alerts, ReqInf, error) {

	var remoteAddr net.Addr
	reqBody, err := json.Marshal(serviceCategory)
	reqInf := ReqInf{CacheHitStatus: CacheHitStatusMiss, RemoteAddr: remoteAddr}
	if err != nil {
		return tc.Alerts{}, reqInf, err
	}
	route := fmt.Sprintf("%s/%d", API_SERVICE_CATEGORIES, id)
	resp, remoteAddr, err := to.request(http.MethodPut, route, reqBody)
	if err != nil {
		return tc.Alerts{}, reqInf, err
	}
	defer resp.Body.Close()
	var alerts tc.Alerts
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return tc.Alerts{}, reqInf, err
	}
	return alerts, reqInf, nil
}

// GetServiceCategories gets a list of service categories by the passed in url values.
func (to *Session) GetServiceCategories(values *url.Values) ([]tc.ServiceCategory, ReqInf, error) {
	url := fmt.Sprintf("%s?%s", API_SERVICE_CATEGORIES, values.Encode())
	resp, remoteAddr, err := to.request(http.MethodGet, url, nil)
	reqInf := ReqInf{CacheHitStatus: CacheHitStatusMiss, RemoteAddr: remoteAddr}
	if err != nil {
		return nil, reqInf, err
	}
	defer resp.Body.Close()

	var data tc.ServiceCategoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, reqInf, err
	}

	return data.Response, reqInf, nil
}

// DeleteServiceCategoryByID deletes a service category by the service category unique numeric id.
func (to *Session) DeleteServiceCategoryByID(id int) (tc.Alerts, ReqInf, error) {
	route := fmt.Sprintf("%s/%d", API_SERVICE_CATEGORIES, id)
	resp, remoteAddr, err := to.request(http.MethodDelete, route, nil)
	reqInf := ReqInf{CacheHitStatus: CacheHitStatusMiss, RemoteAddr: remoteAddr}
	if err != nil {
		return tc.Alerts{}, reqInf, err
	}
	defer resp.Body.Close()
	var alerts tc.Alerts
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return tc.Alerts{}, reqInf, err
	}
	return alerts, reqInf, nil
}
