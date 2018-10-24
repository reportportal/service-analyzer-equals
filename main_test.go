/*
 * Copyright 2018 EPAM Systems
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"bytes"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"gopkg.in/reportportal/commons-go.v1/conf"
	"gopkg.in/reportportal/commons-go.v1/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_DeleteIndex(t *testing.T) {
	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	mux := chi.NewMux()

	mux.Handle("/", server.Handler{H: deleteIndexHandler(NewClient([]string{}, defaultSearchConfig()))})

	req, _ := http.NewRequest(http.MethodDelete, "/", nil)
	mux.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusBadRequest)
	assert.Contains(t, rr.Body.String(), "Index ID is incorrect")
}

func TestClient_CleanIndex(t *testing.T) {
	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	mux := chi.NewMux()

	mux.Handle("/_index/{index_id}/delete", server.Handler{H: cleanIndexHandler(NewClient([]string{}, defaultSearchConfig()))})

	req, _ := http.NewRequest(http.MethodPut, "/_index/xxx/delete", bytes.NewBufferString(`{"ids" : []}`))
	mux.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusBadRequest)
	assert.Contains(t, rr.Body.String(), "Struct validation has failed")
}

func defaultSearchConfig() *SearchConfig {
	sc := &SearchConfig{}
	conf.LoadConfig(sc)
	return sc
}
