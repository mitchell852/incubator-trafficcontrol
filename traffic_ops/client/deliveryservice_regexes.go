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
	"fmt"

	"github.com/apache/trafficcontrol/lib/go-tc"
)

const (
	// See: https://traffic-control-cdn.readthedocs.io/en/latest/api/v3/deliveryservices_id_regexes.html
	API_DS_REGEXES = apiBase + "/deliveryservices/%v/regexes"
)

// GetDeliveryServiceRegexesByDSID gets DeliveryServiceRegexes by a DS id
// also accepts an optional map of query parameters
func (to *Session) GetDeliveryServiceRegexesByDSID(dsID int, params map[string]string) ([]tc.DeliveryServiceIDRegex, ReqInf, error) {
	response := struct {
		Response []tc.DeliveryServiceIDRegex `json:"response"`
	}{}

	reqInf, err := get(to, fmt.Sprintf(API_DS_REGEXES, dsID)+mapToQueryParameters(params), &response, nil)
	if err != nil {
		return []tc.DeliveryServiceIDRegex{}, reqInf, err
	}
	return response.Response, reqInf, nil
}
