package rfc

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

// Package rfc contains functions implementing RFC 7234, 2616, and other RFCs.
// When changing functions, be sure they still conform to the corresponding RFC.
// When adding symbols, document the RFC and section they correspond to.

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apache/trafficcontrol/grove/cacheobj"
	"github.com/apache/trafficcontrol/grove/remapdata"
	"github.com/apache/trafficcontrol/grove/web"

	"github.com/apache/trafficcontrol/lib/go-log"
	"github.com/apache/trafficcontrol/lib/go-rfc"
)

// codeUnderstood returns whether the given response code is understood by this cache. Required by RFC7234§3
func codeUnderstood(code int) bool {
	_, ok := ValidHTTPCodes[code]
	return ok
}

// CanReuseStored checks the constraints in RFC7234§4
func CanReuseStored(reqHeaders http.Header, respHeaders http.Header, reqCacheControl web.CacheControl, respCacheControl web.CacheControl, respReqHeaders http.Header, respReqTime time.Time, respRespTime time.Time, strictRFC bool) remapdata.Reuse {
	// TODO: remove allowed_stale, check in cache manager after revalidate fails? (since RFC7234§4.2.4 prohibits serving stale response unless disconnected).

	if !selectedHeadersMatch(reqHeaders, respReqHeaders, strictRFC) {
		log.Debugf("CanReuseStored false - selected headers don't match\n") // debug
		return remapdata.ReuseCannot
	}

	if !fresh(respHeaders, respCacheControl, respReqTime, respRespTime) {
		allowedStale := allowedStale(respHeaders, reqCacheControl, respCacheControl, respReqTime, respRespTime, strictRFC)
		log.Debugf("CanReuseStored not fresh, allowed stale: %v\n", allowedStale) // debug
		return allowedStale
	}

	if hasPragmaNoCache(reqHeaders) && strictRFC {
		log.Debugf("CanReuseStored MustRevalidate - has pragma no-cache\n")
		return remapdata.ReuseMustRevalidate
	}

	if _, ok := reqCacheControl["no-cache"]; ok && strictRFC {
		log.Debugf("CanReuseStored false - request has cache-control no-cache\n")
		return remapdata.ReuseCannot
	}

	if _, ok := respCacheControl["no-cache"]; ok {
		log.Debugf("CanReuseStored false - response has cache-control no-cache\n")
		return remapdata.ReuseCannot
	}

	if strictRFC && !inMinFresh(respHeaders, reqCacheControl, respCacheControl, respReqTime, respRespTime) {
		return remapdata.ReuseMustRevalidate
	}

	log.Debugf("CanReuseStored true (respCacheControl %+v)\n", respCacheControl)
	return remapdata.ReuseCan
}

// CanReuse is a helper wrapping CanReuseStored, returning a boolean rather than an enum, for when it's known whether MustRevalidate can be used.
func CanReuse(reqHeader http.Header, reqCacheControl web.CacheControl, cacheObj *cacheobj.CacheObj, strictRFC bool, revalidateCanReuse bool) bool {
	canReuse := CanReuseStored(reqHeader, cacheObj.RespHeaders, reqCacheControl, cacheObj.RespCacheControl, cacheObj.ReqHeaders, cacheObj.ReqRespTime, cacheObj.RespRespTime, strictRFC)
	return canReuse == remapdata.ReuseCan || (canReuse == remapdata.ReuseMustRevalidate && revalidateCanReuse)
}

// Fresh checks the constraints in RFC7234§4 via RFC7234§4.2
func fresh(
	respHeaders http.Header,
	respCacheControl web.CacheControl,
	respReqTime time.Time,
	respRespTime time.Time,
) bool {
	freshnessLifetime := getFreshnessLifetime(respHeaders, respCacheControl)
	currentAge := getCurrentAge(respHeaders, respReqTime, respRespTime)
	log.Debugf("Fresh: freshnesslifetime %v currentAge %v\n", freshnessLifetime, currentAge)
	fresh := freshnessLifetime > currentAge
	return fresh
}

// inMinFresh returns whether the given response is within the `min-fresh` request directive. If no `min-fresh` directive exists in the request, `true` is returned.
func inMinFresh(respHeaders http.Header, reqCacheControl web.CacheControl, respCacheControl web.CacheControl, respReqTime time.Time, respRespTime time.Time) bool {
	minFresh, ok := getHTTPDeltaSecondsCacheControl(reqCacheControl, "min-fresh")
	if !ok {
		return true // no min-fresh => within min-fresh
	}
	freshnessLifetime := getFreshnessLifetime(respHeaders, respCacheControl)
	currentAge := getCurrentAge(respHeaders, respReqTime, respRespTime)
	inMinFresh := minFresh < (freshnessLifetime - currentAge)
	log.Debugf("inMinFresh minFresh %v freshnessLifetime %v currentAge %v => %v < (%v - %v) = %v\n", minFresh, freshnessLifetime, currentAge, minFresh, freshnessLifetime, currentAge, inMinFresh)
	return inMinFresh
}

// TODO add warning generation funcs

// AllowedStale checks the constraints in RFC7234§4 via RFC7234§4.2.4
func allowedStale(respHeaders http.Header, reqCacheControl web.CacheControl, respCacheControl web.CacheControl, respReqTime time.Time, respRespTime time.Time, strictRFC bool) remapdata.Reuse {
	// TODO return remapdata.ReuseMustRevalidate where permitted
	_, reqHasMaxAge := reqCacheControl["max-age"]
	_, reqHasMaxStale := reqCacheControl["max-stale"]
	_, respHasMustReval := respCacheControl["must-revalidate"]
	_, respHasProxyReval := respCacheControl["proxy-revalidate"]
	log.Debugf("AllowedStale: reqHasMaxAge %v reqHasMaxStale %v strictRFC %v\n", reqHasMaxAge, reqHasMaxStale, strictRFC)
	if respHasMustReval || respHasProxyReval {
		log.Debugf("AllowedStale: returning mustreval - must-revalidate\n")
		return remapdata.ReuseMustRevalidate
	}
	if strictRFC && reqHasMaxAge && !reqHasMaxStale {
		log.Debugf("AllowedStale: returning can - strictRFC & reqHasMaxAge & !reqHasMaxStale\n")
		return remapdata.ReuseMustRevalidateCanStale
	}
	if _, ok := respCacheControl["no-cache"]; ok {
		log.Debugf("AllowedStale: returning reusecannot - no-cache\n")
		return remapdata.ReuseCannot // TODO verify RFC doesn't allow Revalidate here
	}
	if _, ok := respCacheControl["no-store"]; ok {
		log.Debugf("AllowedStale: returning reusecannot - no-store\n")
		return remapdata.ReuseCannot // TODO verify RFC doesn't allow revalidate here
	}
	if !inMaxStale(respHeaders, respCacheControl, respReqTime, respRespTime) {
		log.Debugf("AllowedStale: returning mustreval - not in max stale\n")
		return remapdata.ReuseMustRevalidate // TODO verify RFC allows
	}
	log.Debugf("AllowedStale: returning can - all preconditions passed\n")
	return remapdata.ReuseMustRevalidateCanStale
}

// InMaxStale returns whether the given response is within the `max-stale` request directive. If no `max-stale` directive exists in the request, `true` is returned.
func inMaxStale(respHeaders http.Header, respCacheControl web.CacheControl, respReqTime time.Time, respRespTime time.Time) bool {
	maxStale, ok := getHTTPDeltaSecondsCacheControl(respCacheControl, "max-stale")
	if !ok {
		// maxStale = 5 // debug
		return true // no max-stale => within max-stale
	}
	freshnessLifetime := getFreshnessLifetime(respHeaders, respCacheControl)
	currentAge := getCurrentAge(respHeaders, respReqTime, respRespTime)
	log.Errorf("DEBUGR InMaxStale maxStale %v freshnessLifetime %v currentAge %v => %v > (%v, %v)\n", maxStale, freshnessLifetime, currentAge, maxStale, currentAge, freshnessLifetime) // DEBUG
	inMaxStale := maxStale > (currentAge - freshnessLifetime)
	return inMaxStale
}

// SelectedHeadersMatch checks the constraints in RFC7234§4.1
// TODO: change caching to key on URL+headers, so multiple requests for the same URL with different vary headers can be cached?
func selectedHeadersMatch(reqHeaders http.Header, respReqHeaders http.Header, strictRFC bool) bool {
	varyHeaders, ok := reqHeaders["vary"]
	if !strictRFC && !ok {
		return true
	}
	if len(varyHeaders) == 0 {
		return true
	}
	varyHeader := varyHeaders[0]

	if varyHeader == "*" {
		return false
	}
	varyHeader = strings.ToLower(varyHeader)
	varyHeaderHeaders := strings.Split(varyHeader, ",")
	for _, header := range varyHeaderHeaders {
		if _, ok := respReqHeaders[header]; !ok {
			return false
		}
	}
	return true
}

// HasPragmaNoCache returns whether the given headers have a `pragma: no-cache` which is to be considered per HTTP/1.1. This specifically returns false if `cache-control` exists, even if `pragma: no-cache` exists, per RFC7234§5.4
func hasPragmaNoCache(reqHeaders http.Header) bool {
	if _, ok := reqHeaders["Cache-Control"]; ok {
		return false
	}
	pragmas, ok := reqHeaders["pragma"]
	if !ok {
		return false
	}
	if len(pragmas) == 0 {
		return false
	}
	pragma := pragmas[0]

	if strings.HasPrefix(pragma, "no-cache") { // RFC7234§5.4 specifically requires no-cache be the first pragma
		return true
	}
	return false
}

// GetHTTPDeltaSeconds is a helper function which gets an HTTP Delta Seconds from the given map (which is typically a `http.Header` or `CacheControl`. Returns false if the given key doesn't exist in the map, or if the value isn't a valid Delta Seconds per RFC2616§3.3.2.
func getHTTPDeltaSeconds(m map[string][]string, key string) (time.Duration, bool) {
	maybeSeconds, ok := m[key]
	if !ok {
		return 0, false
	}
	if len(maybeSeconds) == 0 {
		return 0, false
	}
	maybeSec := maybeSeconds[0]

	seconds, err := strconv.ParseUint(maybeSec, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

// getHTTPDeltaSeconds is a helper function which gets an HTTP Delta Seconds from the given map (which is typically a `http.Header` or `CacheControl`. Returns false if the given key doesn't exist in the map, or if the value isn't a valid Delta Seconds per RFC2616§3.3.2.
func getHTTPDeltaSecondsCacheControl(m map[string]string, key string) (time.Duration, bool) {
	maybeSec, ok := m[key]
	if !ok {
		return 0, false
	}
	seconds, err := strconv.ParseUint(maybeSec, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

const Day = time.Hour * time.Duration(24)

func sinceLastModified(headers http.Header) (time.Duration, bool) {
	lastModified, ok := rfc.GetHTTPDate(headers, "last-modified")
	if !ok {
		return 0, false
	}
	date, ok := rfc.GetHTTPDate(headers, "date")
	if !ok {
		return 0, false
	}
	return date.Sub(lastModified), true
}

// ageValue is used to calculate current_age per RFC7234§4.2.3
func ageValue(respHeaders http.Header) time.Duration {
	s, ok := getHTTPDeltaSeconds(respHeaders, "age")
	if !ok {
		return 0
	}
	return s
}

// dateValue is used to calculate current_age per RFC7234§4.2.3. It returns time, or false if the response had no Date header (in violation of HTTP/1.1).
func dateValue(respHeaders http.Header) (time.Time, bool) {
	return rfc.GetHTTPDate(respHeaders, "date")
}

func apparentAge(respHeaders http.Header, respRespTime time.Time) time.Duration {
	dateValue, ok := dateValue(respHeaders)
	if !ok {
		return 0 // TODO log warning?
	}
	rawAge := respRespTime.Sub(dateValue)
	return time.Duration(math.Max(0.0, float64(rawAge)))
}

func responseDelay(respReqTime time.Time, respRespTime time.Time) time.Duration {
	return respRespTime.Sub(respReqTime)
}

func correctedAgeValue(respHeaders http.Header, respReqTime time.Time, respRespTime time.Time) time.Duration {
	return ageValue(respHeaders) + responseDelay(respReqTime, respRespTime)
}

func correctedInitialAge(respHeaders http.Header, respReqTime time.Time, respRespTime time.Time) time.Duration {
	return time.Duration(math.Max(float64(apparentAge(respHeaders, respRespTime)), float64(correctedAgeValue(respHeaders, respReqTime, respRespTime))))
}

func residentTime(respRespTime time.Time) time.Duration {
	return time.Now().Sub(respRespTime)
}

func getCurrentAge(respHeaders http.Header, respReqTime time.Time, respRespTime time.Time) time.Duration {
	correctedInitial := correctedInitialAge(respHeaders, respReqTime, respRespTime)
	resident := residentTime(respRespTime)
	log.Debugf("getCurrentAge: correctedInitialAge %v residentTime %v\n", correctedInitial, resident)
	return correctedInitial + resident
}
