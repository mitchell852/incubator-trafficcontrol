package cdnnotification

/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/apache/trafficcontrol/lib/go-util"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/dbhelpers"
	"net/http"

	"github.com/apache/trafficcontrol/lib/go-tc"
	"github.com/apache/trafficcontrol/traffic_ops/traffic_ops_golang/api"
)

const readQuery = `
SELECT cn.cdn, 
	cn.username, 
	cn.notification 
FROM cdn_notification as cn
FULL JOIN cdn ON cdn.name = cn.cdn
FULL JOIN tm_user ON tm_user.username = cn.username
`

const insertQuery = `
INSERT INTO cdn_notification (cdn, username, notification)
VALUES ($1, $2, $3)
RETURNING cdn_notification.cdn,
          cdn_notification.username,
          cdn_notification.notification
`

const deleteQuery = `
DELETE FROM cdn_notification
WHERE cdn_notification.cdn = $1
RETURNING cdn_notification.cdn,
          cdn_notification.username,
          cdn_notification.notification
`

// Read is the handler for GET requests to /cdn_notifications.
func Read(w http.ResponseWriter, r *http.Request) {
	inf, userErr, sysErr, errCode := api.NewInfo(r, nil, nil)
	tx := inf.Tx.Tx
	if userErr != nil || sysErr != nil {
		api.HandleErr(w, r, tx, errCode, userErr, sysErr)
		return
	}
	defer inf.Close()

	cdnNotifications := []tc.CDNNotification{}

	queryParamsToQueryCols := map[string]dbhelpers.WhereColumnInfo{
		"cdn":      dbhelpers.WhereColumnInfo{"cdn.name", nil},
		"username": dbhelpers.WhereColumnInfo{"tm_user.username", nil},
	}

	where, orderBy, pagination, queryValues, errs := dbhelpers.BuildWhereAndOrderByAndPagination(inf.Params, queryParamsToQueryCols)
	if len(errs) > 0 {
		sysErr = util.JoinErrs(errs)
		errCode = http.StatusBadRequest
		api.HandleErr(w, r, tx, errCode, nil, sysErr)
		return
	}

	query := readQuery + where + orderBy + pagination
	rows, err := inf.Tx.NamedQuery(query, queryValues)
	if err != nil {
		userErr, sysErr, errCode = api.ParseDBError(err)
		if sysErr != nil {
			sysErr = fmt.Errorf("notification read query: %v", sysErr)
		}

		api.HandleErr(w, r, tx, errCode, userErr, sysErr)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var n tc.CDNNotification
		if err = rows.Scan(&n.CDN, &n.Username, &n.Notification); err != nil {
			api.HandleErr(w, r, tx, http.StatusInternalServerError, nil, errors.New("scanning cdn notifications: "+err.Error()))
			return
		}
		cdnNotifications = append(cdnNotifications, n)
	}

	api.WriteResp(w, r, cdnNotifications)
}

// Create is the handler for POST requests to /cdn_notifications.
func Create(w http.ResponseWriter, r *http.Request) {
	inf, sysErr, userErr, errCode := api.NewInfo(r, nil, nil)
	tx := inf.Tx.Tx
	if sysErr != nil || userErr != nil {
		api.HandleErr(w, r, tx, errCode, userErr, sysErr)
		return
	}
	defer inf.Close()

	var n tc.CDNNotification
	if userErr = api.Parse(r.Body, tx, &n); userErr != nil {
		api.HandleErr(w, r, tx, http.StatusBadRequest, userErr, nil)
		return
	}

	err := tx.QueryRow(insertQuery, n.CDN, inf.User.UserName, n.Notification).Scan(&n.CDN, &n.Username, &n.Notification)
	if err != nil {
		userErr, sysErr, errCode = api.ParseDBError(err)
		api.HandleErr(w, r, tx, errCode, userErr, sysErr)
		return
	}

	changeLogMsg := fmt.Sprintf("CDN_NOTIFICATION: %s, CDN: %s, USER: %s, ACTION: Created", *n.CDN, *n.Username, *n.Notification)
	api.CreateChangeLogRawTx(api.ApiChange, changeLogMsg, inf.User, tx)

	alertMsg := fmt.Sprintf("CDN notification created [ User = %s ] for CDN: %s", *n.Username, *n.CDN)
	api.WriteRespAlertObj(w, r, tc.SuccessLevel, alertMsg, n)
}

// Delete is the handler for DELETE requests to /cdn_notifications.
func Delete(w http.ResponseWriter, r *http.Request) {
	inf, sysErr, userErr, errCode := api.NewInfo(r, []string{"cdn"}, nil)
	tx := inf.Tx.Tx
	if sysErr != nil || userErr != nil {
		api.HandleErr(w, r, tx, errCode, userErr, sysErr)
		return
	}
	defer inf.Close()

	alert, respObj, userErr, sysErr, statusCode := deleteCDNNotification(inf)
	if userErr != nil || sysErr != nil {
		api.HandleErr(w, r, tx, statusCode, userErr, sysErr)
		return
	}

	api.WriteRespAlertObj(w, r, tc.SuccessLevel, alert.Text, respObj)
}

func deleteCDNNotification(inf *api.APIInfo) (tc.Alert, tc.CDNNotification, error, error, int) {
	var userErr error
	var sysErr error
	var statusCode = http.StatusOK
	var alert tc.Alert
	var result tc.CDNNotification

	err := inf.Tx.Tx.QueryRow(deleteQuery, inf.Params["cdn"]).Scan(&result.CDN, &result.Username, &result.Notification)
	if err != nil {
		if err == sql.ErrNoRows {
			userErr = fmt.Errorf("No CDN Notification for %s", inf.Params["cdn"])
			statusCode = http.StatusNotFound
		} else {
			userErr, sysErr, statusCode = api.ParseDBError(err)
		}

		return alert, result, userErr, sysErr, statusCode
	}

	changeLogMsg := fmt.Sprintf("CDN_NOTIFICATION: %s, CDN: %s, USER: %s, ACTION: Deleted", *result.CDN, *result.Username, *result.Notification)
	api.CreateChangeLogRawTx(api.ApiChange, changeLogMsg, inf.User, inf.Tx.Tx)

	alertMsg := fmt.Sprintf("CDN notification deleted [ User = %s ] for CDN: %s", *result.Username, *result.CDN)
	alert = tc.Alert{
		Level: tc.SuccessLevel.String(),
		Text:  alertMsg,
	}

	return alert, result, userErr, sysErr, statusCode
}
