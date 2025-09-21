package local_schema

import (
	"fmt"
	"github.com/KoNekoD/gormite/pkg/assets"
	"github.com/KoNekoD/gormite/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"slices"
)

func (s *store) introspectTables() error {
	keys := maps.Keys(s.objectsMap)
	slices.Sort(keys)

	for _, objectName := range keys {
		if err := handleMappingObject(objectName, s); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (s *store) introspectSequences() {
	for _, table := range s.tables {
		pk := table.GetPrimaryKey()

		if len(pk.GetColumns()) != 1 {
			continue
		}

		typeValid := false
		for _, s := range pk.GetColumns() {
			column := table.GetColumn(pk.GetColumn(s).GetName())
			if _, ok := column.GetColumnType().(*types.IntegerType); ok {
				typeValid = true
			}
		}

		if !typeValid {
			continue
		}

		seqName := fmt.Sprintf("%s__id__seq", table.GetName())

		s.sequences = append(s.sequences, assets.NewSequence(seqName))
	}
}
