package local_schema

import (
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/dtos"
	"github.com/pkg/errors"
)

func IntrospectLocalSchema(path string) (*assets.Schema, error) {
	s := newStore(path)
	s.namespaces = append(s.namespaces, "public")

	var err error

	s.config, err = dtos.NewConfigData(path)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err = s.collectAst(); err != nil {
		return nil, errors.WithStack(err)
	}

	if err = s.introspectTables(); err != nil {
		return nil, errors.WithStack(err)
	}

	s.introspectSequences()

	return assets.NewSchema(s.tables, s.sequences, s.schemaConfig, s.namespaces), nil
}
