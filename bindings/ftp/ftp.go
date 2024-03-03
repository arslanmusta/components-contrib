package ftp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dapr/components-contrib/bindings"
	commonutils "github.com/dapr/components-contrib/common/utils"
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
	default:
		return nil, fmt.Errorf("ftp binding error. unsupported operation %s", req.Operation)
	}
}

// Operations implements bindings.OutputBinding.
func (f *Ftp) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		bindings.CreateOperation,
	}
}

type ftpMetadata struct {
	RootPath string `json:"rootPath"`
	Server   string `json:"server"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type createResponse struct {
	FileName string `json:"fileName"`
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

func (f *Ftp) create(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	metadata, err := f.metadata.mergeWithRequestMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error merging metadata: %w", err)
	}

	// d, err := strconv.Unquote(string(req.Data))
	// if err == nil {
	// 	req.Data = []byte(d)
	// }

	// decoded, err := base64.RawStdEncoding.DecodeString(string(req.Data))
	// if err == nil {
	// 	req.Data = decoded
	// }

	r := strings.NewReader(commonutils.Unquote(req.Data))

	filename := req.Metadata["filename"]
	if filename == "" {
		return nil, fmt.Errorf("ftp binding error: filename is empty")
	}

	absPath, relPath, err := getSecureAbsRelPath(f.metadata.RootPath, filename)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path for file %s: %w", filename, err)
	}

	c, err := ftp.Dial(metadata.Server)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: connection error to %s: %w", metadata.Server, err)
	}

	err = c.Login(metadata.User, metadata.Password)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: login error with user: %s: %w", metadata.User, err)
	}

	err = c.Stor(absPath, r)
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: store error %w", err)
	}

	err = c.Quit()
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: quit error %w", err)
	}

	jsonResponse, err := json.Marshal(createResponse{
		FileName: relPath,
	})
	if err != nil {
		return nil, fmt.Errorf("ftp binding error: error encoding response as JSON: %w", err)
	}

	return &bindings.InvokeResponse{
		Data: jsonResponse,
	}, nil
}

func (metadata ftpMetadata) mergeWithRequestMetadata(req *bindings.InvokeRequest) (ftpMetadata, error) {
	merged := metadata

	return merged, nil
}

func getSecureAbsRelPath(rootPath string, filename string) (absPath string, relPath string, err error) {
	absPath, err = securejoin.SecureJoin(rootPath, filename)
	if err != nil {
		return
	}
	relPath, err = filepath.Rel(rootPath, absPath)
	if err != nil {
		return
	}

	return
}
