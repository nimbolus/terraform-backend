package tfcontext

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

var rootSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type:       "terraform",
			LabelNames: nil,
		},
	},
}

var terraformBlockSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type:       "backend",
			LabelNames: []string{"name"},
		},
	},
}

var backendSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "address"},
		{Name: "username"},
		{Name: "password"},
	},
}

func files(dir string) ([]string, error) {
	infos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, info := range infos {
		if info.IsDir() {
			continue
		}

		name := info.Name()
		ext := filepath.Ext(name)
		if ext != ".tf" {
			continue
		}

		fullPath := filepath.Join(dir, name)
		files = append(files, fullPath)
	}

	return files, nil
}

type BackendConfig struct {
	Address  string
	Username string
	Password string
}

func readAttribute(attrs hcl.Attributes, name string) (string, error) {
	raw, ok := attrs[name]
	if !ok {
		return "", nil
	}

	val, err := raw.Expr.Value(nil)
	if err != nil {
		return "", err
	}

	return val.AsString(), nil
}

func FindBackendBlock(dir string) (*hcl.Block, string, error) {
	parser := hclparse.NewParser()

	tfFiles, err := files(dir)
	if err != nil {
		return nil, "", err
	}

	var file *hcl.File
	for _, filename := range tfFiles {
		b, err := os.ReadFile(filename)
		if err != nil {
			return nil, "", err
		}

		file, _ = parser.ParseHCL(b, filename)
		if file == nil {
			continue
		}

		content, _, _ := file.Body.PartialContent(rootSchema)
		for _, block := range content.Blocks {
			if block.Type != "terraform" {
				continue
			}

			content, _, _ := block.Body.PartialContent(terraformBlockSchema)
			for _, innerBlock := range content.Blocks {
				if innerBlock.Type == "backend" {
					return innerBlock, filename, nil
				}
			}
		}
	}

	return nil, "", errors.New("backend block not found")
}

func FindBackend(dir string) (*BackendConfig, error) {
	backend, _, err := FindBackendBlock(dir)
	if err != nil {
		return nil, err
	}

	if backend.Labels[0] != "http" {
		return nil, errors.New("not using http backend")
	}

	content, _, _ := backend.Body.PartialContent(backendSchema)
	address, err := readAttribute(content.Attributes, "address")
	if err != nil {
		return nil, err
	}
	username, err := readAttribute(content.Attributes, "username")
	if err != nil {
		return nil, err
	}
	password, err := readAttribute(content.Attributes, "password")
	if err != nil {
		return nil, err
	}

	return &BackendConfig{
		Address:  address,
		Username: username,
		Password: password,
	}, nil
}
