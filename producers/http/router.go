// Copyright 2016 Mesosphere, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"net/http"

	"github.com/gorilla/mux"
)

func loadRoutes(routes []Route, router *mux.Router, p *producerImpl) {
	for _, route := range routes {
		httpLog.Debugf("http producer: establishing endpoint %s at %s", route.Name, route.Path)
		var handler http.Handler

		handler = route.HandlerFunc(p)
		handler = logger(handler, route.Name)

		router.NewRoute().
			Methods(route.Method).
			Path(route.Path).
			Name(route.Name).
			Handler(handler)
	}
}

// NewRouter iterates over a slice of Route types and creates them
// in gorilla/mux.
func newRouter(p *producerImpl) *mux.Router {

	router := mux.NewRouter().StrictSlash(true)
	if p.config.DCOSRole == "agent" {
		loadRoutes(agentRoutes, router, p)
	}

	loadRoutes(routes, router, p)

	return router
}
