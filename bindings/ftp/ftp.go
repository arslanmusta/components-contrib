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
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	kitmd "github.com/dapr/kit/metadata"

	"github.com/jlaffaye/ftp"
)

type Ftp struct {
	metadata *ftpMetadata
	logger   logger.Logger
}

// Invoke implements bindings.OutputBinding.
func (f *Ftp) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	switch req.Operation {
	case bindings.CreateOperation:
		return f.create(ctx, req)
	case bindings.ListOperation:
		return f.list(ctx, req)
	case bindings.GetOperation:
		return f.get(ctx, req)
	case bindings.DeleteOperation:
		return f.delete(ctx, req)
	default:
		return nil, fmt.Errorf("ftp binding error. unsupported operation %s", req.Operation)
	}
}

// Operations implements bindings.OutputBinding.
func (f *Ftp) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		bindings.CreateOperation,
		bindings.ListOperation,
		bindings.GetOperation,
		bindings.DeleteOperation,
	}
}

type ftpMetadata struct {
	RootPath  string `json:"rootPath"`
	Server    string `json:"server"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Directory string `json:"directory"`
}

type createResponse struct {
	FileName string `json:"fileName"`
}

type listResponse struct {
	Directory string     `json:"directory"`
	FileInfos []fileInfo `json:"fileInfos"`
}

type fileInfo struct {
	Filename string `json:"filename"`
	FileType string `json:"filetype"`
}

func (f *Ftp) Init(_ context.Context, metadata bindings.Metadata) error {
	m, err := f.parseMetadata(metadata)
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	f.metadata = m

	return nil
}

func (f *Ftp) parseMetadata(md bindings.Metadata) (*ftpMetadata, error) {
	var m ftpMetadata
	err := kitmd.DecodeMetadata(md.Properties, &m)
	if err != nil {
		return nil, err
	}

	return &m, err
}

func NewFtp(logger logger.Logger) bindings.OutputBinding {
	return &Ftp{logger: logger}
}

func (f *Ftp) create(_ context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	metadata, err := f.metadata.mergeWithRequestMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error merging metadata: %w", err)
	}

	r := strings.NewReader(string(req.Data))

	filename := req.Metadata["Filename"]
	if filename == "" {
		return nil, fmt.Errorf("ftp binding error: filename is empty")
	}

	absPath, dir, exactFilename, err := getSecureDirAndFilename(f.metadata.RootPath, filename)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: getting directory and file name for %s %s: %w", f.metadata.RootPath, filename, err)
	}

	c, err := ftp.Dial(metadata.Server)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: connection error to %s: %w", metadata.Server, err)
	}

	err = c.Login(metadata.User, metadata.Password)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: login error with user: %s: %w", metadata.User, err)
	}

	err = c.ChangeDir(dir)
	if err != nil {
		err = c.MakeDir(dir)
		if err != nil {
			return nil, fmt.Errorf("ftp binding error: directory create error for %s: %w", dir, err)
		}
		err = c.ChangeDir(dir)
		if err != nil {
			return nil, fmt.Errorf("ftp binding error: directory change error for %s: %w", dir, err)
		}
	}

	err = c.Stor(exactFilename, r)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: store error %w", err)
	}

	err = c.Quit()
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: quit error %w", err)
	}

	jsonResponse, err := json.Marshal(createResponse{
		FileName: absPath,
	})
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error encoding response as JSON: %w", err)
	}

	return &bindings.InvokeResponse{
		Data: jsonResponse,
	}, nil
}

func (f *Ftp) list(_ context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	metadata, err := f.metadata.mergeWithRequestMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error merging metadata: %w", err)
	}

	c, err := ftp.Dial(metadata.Server)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: connection error to %s: %w", metadata.Server, err)
	}

	err = c.Login(metadata.User, metadata.Password)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: login error with user: %s: %w", metadata.User, err)
	}

	directory := metadata.Directory
	dir, err := getSecureDir(metadata.RootPath, directory)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: getting directory for %s : %w", directory, err)
	}

	entries, err := c.List(dir)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error. directory list error %s: %w", dir, err)
	}

	err = c.Quit()
	if err != nil {
		return nil, fmt.Errorf("ftp binding error. ftp quit error %s: %w", dir, err)
	}

	response := listResponse{
		Directory: dir,
	}

	for _, entry := range entries {
		response.FileInfos = append(response.FileInfos, fileInfo{
			Filename: entry.Name,
			FileType: entry.Type.String(),
		})
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error encoding response as JSON: %w", err)
	}

	return &bindings.InvokeResponse{
		Data: jsonResponse,
	}, nil
}

func (f *Ftp) get(_ context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	metadata, err := f.metadata.mergeWithRequestMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error merging metadata: %w", err)
	}

	filename := req.Metadata["Filename"]
	if filename == "" {
		return nil, fmt.Errorf("ftp binding error: filename is empty")
	}

	_, dir, exactFilename, err := getSecureDirAndFilename(f.metadata.RootPath, filename)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: getting directory and file name for %s %s: %w", f.metadata.RootPath, filename, err)
	}

	c, err := ftp.Dial(metadata.Server)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: connection error to %s: %w", metadata.Server, err)
	}

	err = c.Login(metadata.User, metadata.Password)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: login error with user: %s: %w", metadata.User, err)
	}

	err = c.ChangeDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: directory change error for %s: %w", dir, err)
	}

	res, err := c.Retr(exactFilename)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: retrieve error fpr: %s: %w", filename, err)
	}
	defer res.Close()

	err = c.Quit()
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: quit error %w", err)
	}

	buf, err := io.ReadAll(res)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: retrieve error fpr: %s: %w", filename, err)
	}

	return &bindings.InvokeResponse{
		Data: buf,
	}, nil
}

func (f *Ftp) delete(_ context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	metadata, err := f.metadata.mergeWithRequestMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error merging metadata: %w", err)
	}

	filename := req.Metadata["Filename"]
	if filename == "" {
		return nil, fmt.Errorf("ftp binding error: filename is empty")
	}

	_, dir, exactFilename, err := getSecureDirAndFilename(f.metadata.RootPath, filename)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: getting directory and file name for %s %s: %w", f.metadata.RootPath, filename, err)
	}

	c, err := ftp.Dial(metadata.Server)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: connection error to %s: %w", metadata.Server, err)
	}

	err = c.Login(metadata.User, metadata.Password)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: login error with user: %s: %w", metadata.User, err)
	}

	err = c.ChangeDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: directory change error for %s: %w", dir, err)
	}

	err = c.Delete(exactFilename)
	if err != nil {
		return nil, fmt.Errorf(("ftp binding error: file delete error for %s: %w"), exactFilename, err)
	}

	err = c.Quit()
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: quit error %w", err)
	}

	return &bindings.InvokeResponse{}, nil
}

func (metadata ftpMetadata) mergeWithRequestMetadata(req *bindings.InvokeRequest) (ftpMetadata, error) {
	merged := metadata

	if val, ok := req.Metadata["Directory"]; ok && val != "" {
		merged.Directory = val
	}

	return merged, nil
}

func getSecureDirAndFilename(rootPath string, filename string) (absPath string, dir string, exactFilename string, err error) {
	rootPath, err = securejoin.SecureJoin(".", rootPath)
	if err != nil {
		return
	}
	absPath, err = securejoin.SecureJoin(rootPath, filename)
	if err != nil {
		return
	}
	dir = filepath.Dir(absPath)
	exactFilename = filepath.Base(absPath)

	return
}

func getSecureDir(rootPath string, directory string) (secureDirectory string, err error) {
	rootPath, err = securejoin.SecureJoin(".", rootPath)
	if err != nil {
		return
	}
	secureDirectory, err = securejoin.SecureJoin(rootPath, directory)

	return
}

// GetComponentMetadata returns the metadata of the component.
func (f *Ftp) GetComponentMetadata() (metadataInfo metadata.MetadataMap) {
	metadataStruct := ftpMetadata{}
	metadata.GetMetadataInfoFromStructType(reflect.TypeOf(metadataStruct), &metadataInfo, metadata.BindingType)
	return
}
