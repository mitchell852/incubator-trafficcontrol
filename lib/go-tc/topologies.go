package tc

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

// Topology holds the name and set of TopologyNodes that comprise a flexible topology.
type Topology struct {
	Description string           `json:"description" db:"description"`
	Name        string           `json:"name" db:"name"`
	Nodes       *[]*TopologyNode `json:"nodes"`
	LastUpdated *TimeNoMod       `json:"lastUpdated" db:"last_updated"`
}

// TopologyNode holds a reference to a cachegroup and the indices of up to 2 parent
// nodes in the same topology's array of nodes.
type TopologyNode struct {
	Id          int        `json:"-" db:"id"`
	Cachegroup  string     `json:"cachegroup" db:"cachegroup"`
	Parents     []int      `json:"parents"`
	LastUpdated *TimeNoMod `json:"-" db:"last_updated"`
}

// TopologiesResponse models the JSON object returned for a list of topologies in a response.
type TopologyResponse struct {
	Response Topology `json:"response"`
	Alerts
}

// TopologiesResponse models the JSON object returned for a single topology in a response.
type TopologiesResponse struct {
	Response []Topology `json:"response"`
	Alerts
}