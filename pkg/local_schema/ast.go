package local_schema

import (
	"github.com/pkg/errors"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/exp/maps"
	"slices"
	"strings"
)

func (s *store) collectAst() error {
	mappingKeys := maps.Keys(s.config.Gormite.Orm.Mapping)
	slices.Sort(mappingKeys)

	for _, mappingKey := range mappingKeys {
		if err := s.collectMappingKeyAst(mappingKey); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (s *store) collectMappingKeyAst(mappingKey string) error {
	mapping := s.config.Gormite.Orm.Mapping[mappingKey]

	parsed, err := parser.ParseDir(token.NewFileSet(), mapping.Dir, nil, parser.ParseComments)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(parsed) != 1 {
		return errors.Errorf("expected 1 package, got %d", len(parsed))
	}

	firstKey := maps.Keys(parsed)[0]

	parsedPackage := parsed[firstKey]

	for fileName, fileData := range parsedPackage.Files {
		if err := s.collectMappingKeyFileAst(fileName, fileData); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (s *store) collectMappingKeyFileAst(fileName string, fileData *ast.File) error {
	for objectName, object := range fileData.Scope.Objects {
		if _, ok := object.Decl.(*ast.FuncDecl); ok {
			continue // ignore functions
		}

		if _, ok := s.objectsMap[objectName]; ok {
			return errors.Errorf("duplicate object %s", objectName)
		}

		s.objectsMap[objectName] = object

		if _, ok := s.structNamesIdentsMap[objectName]; ok {
			return errors.Errorf("duplicate struct %s", objectName)
		}
		s.structNamesIdentsMap[object.Name] = object.Decl.(*ast.TypeSpec).Name
	}

	for _, comment := range fileData.Comments {
		commentStr := strings.Trim(comment.Text(), "//")
		commentParts := strings.Split(commentStr, " ")

		if len(commentParts) < 2 {
			return errors.Errorf("expected 2 parts in comment %s", commentStr)
		}

		s.namesMap[commentParts[0]] = commentParts[1]
	}

	s.importsMap[fileName] = fileData.Imports

	return nil
}
