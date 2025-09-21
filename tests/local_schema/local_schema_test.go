package local_schema

import (
	"github.com/KoNekoD/gormite/pkg/local_schema"
	"github.com/KoNekoD/gormite/pkg/platforms"
	"github.com/pkg/errors"
	"testing"
)

func TestIntrospectLocalSchema(t *testing.T) {
	abstractPlatform := &platforms.AbstractPlatform{}
	maxIdentifierLength := abstractPlatform.GetMaxIdentifierLength()

	newSchema, err := local_schema.IntrospectLocalSchema("resources/gormite.yaml")
	if err != nil {
		panic(errors.Wrap(err, "failed to introspect local schema"))
	}

	// Check if introspected local scheme tables has correct index names lengths
	for _, table := range newSchema.GetTables() {
		for _, index := range table.GetIndexes() {
			if len(index.GetName()) > maxIdentifierLength {
				t.Fatalf("index %s of table %s exceeded max identifier length", index.GetName(), table.GetName())
			}
		}
	}
}
