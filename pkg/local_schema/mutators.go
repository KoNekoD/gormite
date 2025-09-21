package local_schema

import (
	"fmt"
	"github.com/KoNekoD/ptrs/pkg/ptrs"
	"github.com/pkg/errors"
	"strings"
)

func applyMetadataMutatorsForNewColumn(columnTagsData *columnData, bag *tableBag) error {
	if columnTagsData.IsForeignKey {
		options := map[string]any{}
		if columnTagsData.OnUpdate != nil {
			options["onUpdate"] = columnTagsData.OnUpdate
		}
		if columnTagsData.OnDelete != nil {
			options["onDelete"] = columnTagsData.OnDelete
		}

		name := getName(bag.store, columnTagsData.TypeName)
		bag.table.AddForeignKeyConstraint(name, []string{columnTagsData.ColumnName}, []string{"id"}, options, nil)
	}

	if columnTagsData.IsPrimaryKey {
		bag.primaryKeys = append(bag.primaryKeys, columnTagsData.ColumnName)
	}

	if columnTagsData.IsUnique {
		uniqNames := strings.Split(*columnTagsData.UniqueName, ",")
		for _, uniqNameItem := range uniqNames {
			uniqNameItem = strings.TrimSpace(uniqNameItem)
			if _, hasUniqMapKey := bag.uniqColumnsMap[uniqNameItem]; !hasUniqMapKey {
				bag.uniqColumnsMap[uniqNameItem] = make([]string, 0)
			}
			bag.uniqColumnsMap[uniqNameItem] = append(bag.uniqColumnsMap[uniqNameItem], columnTagsData.ColumnName)
		}
	}

	if columnTagsData.IsUniqueCondition {
		conditions := strings.Split(*columnTagsData.UniqueCondition, ";")
		for _, condition := range conditions {
			conditionParts := strings.Split(condition, ":")
			if len(conditionParts) != 2 {
				return errors.Errorf("invalid uniq condition %s", condition)
			}

			bag.uniqConditionsMap[conditionParts[0]] = conditionParts[1]
		}
	}

	if columnTagsData.IsIndex {
		indexNames := strings.Split(*columnTagsData.IndexName, ",")
		for _, indexNameItem := range indexNames {
			indexNameItem = strings.TrimSpace(indexNameItem)
			if _, hasIndexMapKey := bag.indexColumnsMap[indexNameItem]; !hasIndexMapKey {
				bag.indexColumnsMap[indexNameItem] = make([]string, 0)
			}
			bag.indexColumnsMap[indexNameItem] = append(bag.indexColumnsMap[indexNameItem], columnTagsData.ColumnName)
		}
	}

	if columnTagsData.IsIndexCondition {
		conditions := strings.Split(*columnTagsData.IndexCondition, ";")
		for _, condition := range conditions {
			conditionParts := strings.Split(condition, ":")
			if len(conditionParts) != 2 {
				return errors.Errorf("invalid index condition %s", condition)
			}

			bag.indexConditionsMap[conditionParts[0]] = conditionParts[1]
		}
	}

	return nil
}

func applyMetadataMutatorsAfterColumnsIntrospection(bag *tableBag) error {
	for indexName, columns := range bag.indexColumnsMap {
		options := make(map[string]any)

		if v, ok := bag.indexConditionsMap[indexName]; ok {
			options["where"] = v
		}

		bag.table.AddIndex(columns, &indexName, make([]string, 0), options)
	}

	for uniqPseudoName, columns := range bag.uniqColumnsMap {
		uniqIdxName := fmt.Sprintf("idx__%s__%s__uniq", bag.table.GetName(), strings.Join(columns, "_"))

		options := make(map[string]any)

		if v, ok := bag.uniqConditionsMap[uniqPseudoName]; ok {
			options["where"] = v
		}

		bag.table.AddUniqueIndex(columns, &uniqIdxName, options)
	}

	if len(bag.primaryKeys) == 0 {
		return errors.Errorf("primary key of table %s not found", bag.table.GetName())
	}

	bag.table.SetPrimaryKey(bag.primaryKeys, ptrs.AsPtr(fmt.Sprintf("%s_pkey", bag.table.GetName())))

	return nil
}
