package atscfg

/*
 Copyright 2019, Comcast Corporation. This software and its contents are
 Comcast confidential and proprietary. It cannot be used, disclosed, or
 distributed without Comcast's prior written permission. Modification of this
 software is only allowed at the direction of Comcast Corporation. All allowed
 modifications must be provided to Comcast Corporation.
*/

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/trafficcontrol/lib/go-tc"
)

var underscoreDigitRegex = regexp.MustCompile(`__\d+$`)

const ComcastTeakConfigFileName = `to_ext_teak.config`

type ComcastToExtTeakConfigServer struct {
	CachegroupID int
	HostName     string
	IPAddress    string
	Profile      string
	Status       string
	Type         string
}

func ServerToComcastToExtTeakConfigServer(sv tc.Server) ComcastToExtTeakConfigServer {
	return ComcastToExtTeakConfigServer{
		CachegroupID: sv.CachegroupID,
		HostName:     sv.HostName,
		IPAddress:    sv.IPAddress,
		Profile:      sv.Profile,
		Status:       sv.Status,
		Type:         sv.Type,
	}
}

func ServersToComcastToExtTeakConfigServers(servers []tc.Server) []ComcastToExtTeakConfigServer {
	cServers := []ComcastToExtTeakConfigServer{}
	for _, sv := range servers {
		cServers = append(cServers, ServerToComcastToExtTeakConfigServer(sv))
	}
	return cServers
}

func MakeComcastToExtTeakConfig(
	server ComcastToExtTeakConfigServer,
	toToolName string, // tm.toolname global parameter (TODO: cache itself?)
	toURL string, // tm.url global parameter (TODO: cache itself?)
	serverProfileParameters []tc.Parameter,
	serverCacheGroupParameters []tc.Parameter,
	servers []ComcastToExtTeakConfigServer, // may be only teaks; but this will filter reported/online teaks, if all servers are passed.
) string {
	fileParams := []tc.Parameter{}
	for _, param := range serverProfileParameters {
		if param.ConfigFile != ComcastTeakConfigFileName {
			continue
		}
		fileParams = append(fileParams, param)
	}

	nodeOrderStr := ""

	allParams := map[string]string{} // map[paramName]paramVal
	for _, param := range fileParams {
		allParams[param.Name] = param.Value
	}
	for _, param := range serverCacheGroupParameters {
		if param.ConfigFile != ComcastTeakConfigFileName {
			continue
		}
		if param.Name == `cgw.TeakCatalogUrl` {
			continue // JvD note: cgw.teakCatalogUrl will be deleted later.
		}
		if param.Name == `node_order` {
			nodeOrderStr = param.Value
			continue
		}

		paramName := param.Name
		paramName = strings.Replace(paramName, `cgw.`, ``, -1)
		paramName = strings.Replace(paramName, `teakcluster.`, ``, -1)

		paramVal := param.Value

		allParams[paramName] = paramVal
	}

	allParams[`transferIP`] = server.IPAddress

	allParamsSorted := []ComcastTeakNameVal{}
	for name, val := range allParams {
		allParamsSorted = append(allParamsSorted, ComcastTeakNameVal{Name: name, Val: val})
	}

	delim := `=`

	sort.Sort(ComcastTeakNameValSortByName(allParamsSorted))

	text := GenericHeaderComment(server.HostName, toToolName, toURL)

	for _, nameVal := range allParamsSorted {
		name := nameVal.Name
		val := nameVal.Val
		if name == `others` ||
			name == `SubRoutine` ||
			name == `location` {
			continue
		}

		underscoreDigitRegex.ReplaceAllString(name, ``) // $param =~ s/__\d+$//;

		text += name + delim + val + "\n"
	}

	teakServers := []ComcastToExtTeakConfigServer{}
	for _, sv := range servers {
		if sv.Status == string(tc.CacheStatusOffline) || sv.Status == string(tc.CacheStatusAdminDown) {
			continue
		}
		if sv.CachegroupID != server.CachegroupID {
			continue
		}
		if sv.Type != `EDGE_VECTOR` && sv.Type != `EDGE_TEAK` {
			continue
		}
		teakServers = append(teakServers, sv)
	}

	ips := map[string]string{} // map[hostName]ip
	for _, sv := range teakServers {
		ips[sv.HostName] = sv.IPAddress
	}

	hostsInCacheGroup := strings.Split(nodeOrderStr, `:`)
	numNodes := len(hostsInCacheGroup)

	max := 255
	start := 0
	for _, node := range hostsInCacheGroup {
		if strings.TrimSpace(node) == "" {
			continue
		}
		end := start + max
		if numNodes != 0 {
			end = start + max/numNodes
		}
		if end > max {
			end = max
		}
		text += "node=" + strconv.Itoa(start) + "-" + strconv.Itoa(end) + "," + ips[node] + ":" + allParams[`transferPort`] + "," + allParams[`atsPort`] + "\n"
		start = end + 1
	}

	return text
}

type ComcastTeakNameVal struct {
	Name string
	Val  string
}

type ComcastTeakNameValSortByName []ComcastTeakNameVal

func (p ComcastTeakNameValSortByName) Len() int           { return len(p) }
func (p ComcastTeakNameValSortByName) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p ComcastTeakNameValSortByName) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
