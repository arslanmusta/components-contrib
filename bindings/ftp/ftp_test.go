/*
Copyright 2021 The Dapr Authors
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

package ftp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
)

func TestParseMetadata(t *testing.T) {
	t.Run("Has correct metadata", func(t *testing.T) {
		m := bindings.Metadata{}
		// 	RootPath  string `json:"rootPath"`
		// Server    string `json:"server"`
		// Port      string `json:"port"`
		// User      string `json:"user"`
		// Password  string `json:"password"`
		// Directory string `json:"directory"`
		m.Properties = map[string]string{
			"RootPath":  "rootpath",
			"Server":    "server",
			"Port":      "port",
			"Directory": "directory",
		}
		ftp := Ftp{}
		meta, err := ftp.parseMetadata(m)

		require.NoError(t, err)
		assert.Equal(t, "rootpath", meta.RootPath)
		assert.Equal(t, "server", meta.Server)
		assert.Equal(t, "port", meta.Port)
		assert.Equal(t, "directory", meta.Directory)
	})
}

func TestMergeWithRequestMetadata(t *testing.T) {
	t.Run("Has merged metadata", func(t *testing.T) {
		m := bindings.Metadata{}
		m.Properties = map[string]string{
			"RootPath": "rootpath",
			"Server":   "server",
			"Port":     "port",
		}
		ftp := Ftp{}
		meta, err := ftp.parseMetadata(m)

		require.NoError(t, err)
		assert.Equal(t, "rootpath", meta.RootPath)
		assert.Equal(t, "server", meta.Server)
		assert.Equal(t, "port", meta.Port)

		request := bindings.InvokeRequest{}
		request.Metadata = map[string]string{
			"Directory": "directory",
		}

		mergedMeta, err := meta.mergeWithRequestMetadata(&request)

		require.NoError(t, err)
		assert.Equal(t, "directory", mergedMeta.Directory)
	})
}

func TestGetOption(t *testing.T) {
	ftp := NewFtp(logger.NewLogger("ftp")).(*Ftp)
	ftp.metadata = &ftpMetadata{}

	t.Run("return error if filename is missing", func(t *testing.T) {
		r := bindings.InvokeRequest{}
		_, err := ftp.get(context.Background(), &r)
		require.Error(t, err)
	})
}

func TestDeleteOption(t *testing.T) {
	ftp := NewFtp(logger.NewLogger("ftp")).(*Ftp)
	ftp.metadata = &ftpMetadata{}

	t.Run("return error if filename is missing", func(t *testing.T) {
		r := bindings.InvokeRequest{}
		_, err := ftp.delete(context.Background(), &r)
		require.Error(t, err)
	})
}
