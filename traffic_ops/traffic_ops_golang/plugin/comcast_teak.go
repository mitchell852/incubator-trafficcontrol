package plugin

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

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apache/trafficcontrol/lib/go-atscfg"
	"github.com/apache/trafficcontrol/lib/go-rfc"
	"github.com/apache/trafficcontrol/lib/go-tc"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/api"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/ats"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/auth"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/dbhelpers"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/routing/middleware"
)

func init() {
	AddPlugin(10000, Funcs{onRequest: comcast_teak_OnRequest}, "Teak Daemon config file", "0.93")
}

const comcast_teak_PrivLevelRequired = auth.PrivLevelOperations

func comcast_teak_OnRequest(d OnRequestData) IsRequestHandled {
	r := d.R
	w := d.W

	pathParts := strings.Split(d.R.URL.Path, `/`)
	if len(pathParts) != 8 || pathParts[1] != `api` || pathParts[3] != `servers` || pathParts[5] != `configfiles` || pathParts[6] != `ats` {
		return RequestUnhandled
	}

	serverNameOrID := pathParts[4]
	configFileName := pathParts[7]

	if configFileName != atscfg.ComcastTeakConfigFileName {
		return RequestUnhandled
	}

	user, userErr, sysErr, errCode := api.GetUserFromReq(w, r, d.AppCfg.Secrets[0])
	if userErr != nil || sysErr != nil {
		api.HandleErr(w, r, nil, errCode, userErr, sysErr)
		return RequestHandled
	}
	if user.PrivLevel < comcast_teak_PrivLevelRequired {
		api.HandleErr(w, r, nil, http.StatusForbidden, errors.New("Forbidden."), nil)
		return RequestHandled
	}
	api.AddUserToReq(r, user)

	inf, userErr, sysErr, errCode := api.NewInfo(r, nil, nil)
	if userErr != nil || sysErr != nil {
		api.HandleErr(w, r, inf.Tx.Tx, errCode, userErr, sysErr)
		return RequestHandled
	}
	defer inf.Close()

	serverID, err := strconv.Atoi(serverNameOrID)
	if err != nil {
		ok := false
		err := error(nil)
		serverID, ok, err = dbhelpers.GetServerIDFromName(serverNameOrID, inf.Tx.Tx)
		if err != nil {
			api.HandleErr(w, r, inf.Tx.Tx, http.StatusInternalServerError, nil, fmt.Errorf("getting server name from id %v: %v", serverID, err))
			return RequestHandled
		} else if !ok {
			api.HandleErr(w, r, inf.Tx.Tx, http.StatusNotFound, errors.New("getting server ID: server '"+serverNameOrID+"' not found"), nil)
			return RequestHandled
		}
	}

	server, ok, err := comcast_teak_GetToExtTeakConfigServer(inf.Tx.Tx, serverID)
	if err != nil {
		api.HandleErr(w, r, inf.Tx.Tx, http.StatusInternalServerError, nil, fmt.Errorf("getting server name from id %v: %v", serverID, err))
		return RequestHandled
	} else if !ok {
		api.HandleErr(w, r, inf.Tx.Tx, http.StatusNotFound, errors.New("getting server: server not found"), nil)
		return RequestHandled
	}

	toToolName, toURL, err := ats.GetToolNameAndURL(inf.Tx.Tx)
	if err != nil {
		api.HandleErr(w, r, inf.Tx.Tx, http.StatusInternalServerError, nil, errors.New("getting tool name and url: "+err.Error()))
		return RequestHandled
	}

	paramMap, err := ats.GetProfileParamsByName(inf.Tx.Tx, server.Profile, atscfg.ComcastTeakConfigFileName)

	// require a 'location' parameter. Emulates Perl.
	if _, ok := paramMap[`location`]; !ok {
		w.Header().Set(rfc.ContentType, rfc.ApplicationJSON)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"alerts":[{"level":"error","text":"Resource not found."}]}`)) // api.HandleErr works, but this is the exact JSON string Perl prints, where encoding/json sorts the keys.
		return RequestHandled
	}

	params := []tc.Parameter{}
	for name, vals := range paramMap {
		for _, val := range vals {
			params = append(params, tc.Parameter{
				Name:       name,
				Value:      val,
				ConfigFile: atscfg.ComcastTeakConfigFileName,
			})
		}
	}

	locParams, err := comcast_teak_GetCacheGroupParameters(inf.Tx.Tx, server.CachegroupID)
	if err != nil {
		api.HandleErr(w, r, inf.Tx.Tx, http.StatusInternalServerError, nil, errors.New("getting cachegroup parameters: "+err.Error()))
		return RequestHandled
	}

	servers, err := comcast_teak_GetTeakServers(inf.Tx.Tx, server.CachegroupID)
	if err != nil {
		api.HandleErr(w, r, inf.Tx.Tx, http.StatusInternalServerError, nil, errors.New("getting teak servers: "+err.Error()))
		return RequestHandled
	}

	text := atscfg.MakeComcastToExtTeakConfig(server, toToolName, toURL, params, locParams, servers)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(text))
	}
	requestTimeout := middleware.DefaultRequestTimeout
	if d.AppCfg.RequestTimeout != 0 {
		requestTimeout = time.Second * time.Duration(d.AppCfg.RequestTimeout)
	}
	mw := middleware.GetDefault(d.AppCfg.Secrets[0], requestTimeout)
	handler = middleware.Use(handler, mw)
	handler(w, r)

	return RequestHandled
}

func comcast_teak_GetToExtTeakConfigServer(tx *sql.Tx, serverID int) (atscfg.ComcastToExtTeakConfigServer, bool, error) {
	qry := `
SELECT
  s.cachegroup as cachegroup_id,
  s.host_name,
  s.ip_address,
  pr.name as profile_name,
  st.name as status,
  tp.name as type_name
FROM
  server s
  JOIN status st ON s.status = st.id
  JOIN type tp on s.type = tp.id
  JOIN profile pr on s.profile = pr.id
WHERE
  s.id = $1
`

	s := atscfg.ComcastToExtTeakConfigServer{}
	if err := tx.QueryRow(qry, serverID).Scan(&s.CachegroupID, &s.HostName, &s.IPAddress, &s.Profile, &s.Status, &s.Type); err != nil {
		if err == sql.ErrNoRows {
			return atscfg.ComcastToExtTeakConfigServer{}, false, nil
		}
		return atscfg.ComcastToExtTeakConfigServer{}, false, errors.New("querying: " + err.Error())
	}
	return s, true, nil
}

func comcast_teak_GetCacheGroupParameters(tx *sql.Tx, cacheGroupID int) ([]tc.Parameter, error) {
	qry := `
SELECT
  p.config_file,
  p.id,
  p.last_updated,
  p.name,
  p.value,
  p.secure
FROM
  parameter p
  JOIN cachegroup_parameter cgp ON cgp.parameter = p.id
WHERE
  cgp.cachegroup = $1
`

	rows, err := tx.Query(qry, cacheGroupID)
	if err != nil {
		return nil, errors.New("querying: " + err.Error())
	}
	defer rows.Close()

	ps := []tc.Parameter{}
	for rows.Next() {
		p := tc.Parameter{}
		if err := rows.Scan(&p.ConfigFile, &p.ID, &p.LastUpdated, &p.Name, &p.Value, &p.Secure); err != nil {
			return nil, errors.New("scanning: " + err.Error())
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func comcast_teak_GetTeakServers(tx *sql.Tx, cacheGroupID int) ([]atscfg.ComcastToExtTeakConfigServer, error) {
	qry := `
SELECT
  s.cachegroup as cachegroup_id,
  s.host_name,
  s.ip_address,
  st.name as status,
  tp.name as type_name
FROM
  server s
  JOIN status st ON s.status = st.id
  JOIN type tp on s.type = tp.id
  JOIN cachegroup cg on s.cachegroup = cg.id
WHERE
  cg.id = $1
  AND (tp.name = 'EDGE_VECTOR' OR tp.name = 'EDGE_TEAK')
  AND (st.name <> '` + string(tc.CacheStatusOffline) + `' AND st.name <> '` + string(tc.CacheStatusAdminDown) + `')
`

	rows, err := tx.Query(qry, cacheGroupID)
	if err != nil {
		return nil, errors.New("querying: " + err.Error())
	}
	defer rows.Close()

	ss := []atscfg.ComcastToExtTeakConfigServer{}
	for rows.Next() {
		s := atscfg.ComcastToExtTeakConfigServer{}
		if err := rows.Scan(&s.CachegroupID, &s.HostName, &s.IPAddress, &s.Status, &s.Type); err != nil {
			return nil, errors.New("scanning: " + err.Error())
		}
		ss = append(ss, s)
	}
	return ss, nil
}
