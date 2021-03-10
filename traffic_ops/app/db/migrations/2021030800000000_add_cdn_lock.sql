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

-- +goose Up
CREATE TABLE IF NOT EXISTS cdn_lock (
username text NOT NULL,
cdn_name text NOT NULL,
message text,
last_updated timestamp with time zone DEFAULT now() NOT NULL,
CONSTRAINT pk_cdn_lock PRIMARY KEY (cdn_name),
CONSTRAINT fk_lock_cdn FOREIGN KEY (cdn_name) REFERENCES cdn(name) ON DELETE CASCADE ON UPDATE CASCADE,
CONSTRAINT fk_lock_username FOREIGN KEY ("username") REFERENCES tm_user(username) ON DELETE CASCADE ON UPDATE CASCADE
);

DROP TRIGGER IF EXISTS on_update_current_timestamp ON cdn_lock;
CREATE TRIGGER on_update_current_timestamp BEFORE UPDATE ON cdn_lock FOR EACH ROW EXECUTE PROCEDURE on_update_current_timestamp_last_updated();

-- +goose Down
DROP TABLE IF EXISTS cdn_lock;