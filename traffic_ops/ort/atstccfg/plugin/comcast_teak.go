package plugin

/*
 Copyright 2011-2014, Comcast Corporation. This software and its contents are
 Comcast confidential and proprietary. It cannot be used, disclosed, or
 distributed without Comcast's prior written permission. Modification of this
 software is only allowed at the direction of Comcast Corporation. All allowed
 modifications must be provided to Comcast Corporation.
*/

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/apache/trafficcontrol/lib/go-atscfg"
	"github.com/apache/trafficcontrol/lib/go-tc"
	"github.com/apache/trafficcontrol/traffic_ops/ort/atstccfg/config"
	"github.com/apache/trafficcontrol/traffic_ops/ort/atstccfg/toreq"
	"github.com/apache/trafficcontrol/traffic_ops/ort/atstccfg/torequtil"
)

func init() {
	AddPlugin(10000, Funcs{modifyFiles: comcast_teak_ModifyFiles})
}

// TODO: add to meta, based on to_ext_teak.config param, remove the need for a take-n-bake Param

func comcast_teak_ModifyFiles(d ModifyFilesData) []config.ATSConfigFile {
	fileLocation := ""
	for _, param := range d.TOData.ServerParams {
		if param.ConfigFile == atscfg.ComcastTeakConfigFileName && param.Name == `location` {
			fileLocation = param.Value
			break
		}
	}
	if fileLocation == "" {
		return d.Files
	}

	locParams, err := GetCacheGroupParameters(d.Cfg, d.TOData.Server.CachegroupID)
	if err != nil {
		fmt.Println("ERROR getting CacheGroupParameters for server cachegroup " + strconv.Itoa(d.TOData.Server.CachegroupID) + ": " + err.Error())
		os.Exit(config.ExitCodeErrGeneric)
	}

	cfgServer := atscfg.ServerToComcastToExtTeakConfigServer(d.TOData.Server)
	cfgServers := atscfg.ServersToComcastToExtTeakConfigServers(d.TOData.Servers)

	text := atscfg.MakeComcastToExtTeakConfig(cfgServer, d.TOData.TOToolName, d.TOData.TOURL, d.TOData.ServerParams, locParams, cfgServers)

	fi := config.ATSConfigFile{}
	fi.Text = text
	fi.ContentType = "text/plain"
	fi.FileNameOnDisk = atscfg.ComcastTeakConfigFileName
	fi.Location = fileLocation

	d.Files = append(d.Files, fi)
	return d.Files
}

func GetCacheGroupParameters(cfg config.TCCfg, cacheGroupID int) ([]tc.Parameter, error) {
	params := []tc.Parameter{}
	err := torequtil.GetRetry(cfg.NumRetries, "cachegroup_parameters_id_"+strconv.Itoa(cacheGroupID), &params, func(obj interface{}) error {
		toParams, reqInf, err := (*cfg.TOClient.C).GetCacheGroupParameters(cacheGroupID)
		if err != nil {
			return errors.New("getting cachegroup parameters id '" + strconv.Itoa(cacheGroupID) + "' from Traffic Ops '" + toreq.MaybeIPStr(reqInf.RemoteAddr) + "': " + err.Error())
		}
		params := obj.(*[]tc.Parameter)
		for _, cgParam := range toParams {
			*params = append(*params, tc.Parameter{
				ConfigFile:  cgParam.ConfigFile,
				ID:          cgParam.ID,
				LastUpdated: cgParam.LastUpdated,
				Name:        cgParam.Name,
				Secure:      cgParam.Secure,
				Value:       cgParam.Value,
			})
		}
		return nil
	})
	if err != nil {
		return nil, errors.New("getting params cachegroup id '" + strconv.Itoa(cacheGroupID) + "': " + err.Error())
	}
	return params, nil
}
