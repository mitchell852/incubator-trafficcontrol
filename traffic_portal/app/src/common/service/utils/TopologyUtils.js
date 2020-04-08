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

var TopologyUtils = function() {

	let normalizedTopology;

	let flattenNormalizedTopology = function(topologyTree, fromScratch) {
		if (fromScratch) normalizedTopology.nodes = [];
		topologyTree.forEach(function(node) {
			if (node.cachegroup) {
				normalizedTopology.nodes.push({
					cachegroup: node.cachegroup,
					parent: node.parent,
					secParent: node.secParent,
					parents: []
				});
			}
			if (node.children && node.children.length > 0) {
				flattenNormalizedTopology(node.children, false);
			}
		});
	};

	let addNodeIndexes = function() {
		normalizedTopology.nodes.forEach(function(currentNode) {
			let parentNodeIndex = _.findIndex(normalizedTopology.nodes, function(node) { return currentNode.parent === node.cachegroup });
			let secParentNodeIndex = _.findIndex(normalizedTopology.nodes, function(node) { return currentNode.secParent === node.cachegroup });
			if (parentNodeIndex > -1) {
				currentNode.parents.push(parentNodeIndex);
				if (secParentNodeIndex > -1) {
					currentNode.parents.push(secParentNodeIndex);
				}
			}
		});
		normalizedTopology.nodes.forEach(function(currentNode) {
			delete currentNode.parent;
			delete currentNode.secParent;
		});
	};

	this.getNormalizedTopology = function(name, desc, topologyTree) {
		normalizedTopology = {
			name: name,
			desc: desc,
			nodes: []
		};
		flattenNormalizedTopology(topologyTree);
		addNodeIndexes();
		return normalizedTopology;
	};

	this.getTopologyTree = function(topology) {
		let roots = [], // topology items without parents (primary or secondary)
			all = {};

		topology.nodes.forEach(function(node, index) {
			all[index] = node;
		});

		// create children based on parent definitions
		Object.keys(all).forEach(function (guid) {
			let item = all[guid];
			if (!('children' in item)) {
				item.children = []
			}
			if (item.parents.length === 0) {
				item.parent = "";
				item.secParent = "";
				roots.push(item)
			} else if (item.parents[0] in all) {
				let p = all[item.parents[0]]
				if (!('children' in p)) {
					p.children = []
				}
				p.children.push(item);
				// add parent to each node
				item.parent = all[item.parents[0]].cachegroup;
				// add secParent to each node
				if (item.parents.length === 2 && item.parents[1] in all) {
					item.secParent = all[item.parents[1]].cachegroup;
				}
			}
		});

		return [
			{
				children: roots
			}
		];
	};

};

TopologyUtils.$inject = [];
module.exports = TopologyUtils;
