package assets

import (
	"github.com/KoNekoD/gormite/pkg/utils"
	"hash/crc32"
	"strings"
)

// AbstractAsset - The abstract asset allows to reset the name of all assets without publishing this to the public userland.
// This encapsulation hack is necessary to keep a consistent state of the database schema. Say we have a list of tables
// array(tableName => Table(tableName)); if you want to rename the table, you have to make sure this does not get
// recreated during schema migration.
type AbstractAsset struct {
	name string
	// namespace - Namespace of the asset. If none isset the default namespace is assumed.
	namespace string
	quoted    bool
}

type AbstractAssetInterface interface {
	SetName(name string) *AbstractAsset
	IsInDefaultNamespace(defaultNamespaceName string) bool
	GetNamespaceName() string
	GetShortestName(defaultNamespaceName string) string
	IsQuoted() bool
	isIdentifierQuoted(identifier string) bool
	trimQuotes(identifier string) string
	GetName() string
	GetQuotedName(platform AssetsPlatform) string
	generateIdentifierName(
		columnNames []string,
		prefix string,
		maxSize int,
	) string
}

func NewAbstractAsset() *AbstractAsset {
	return &AbstractAsset{}
}

// SetName - Sets the name of this asset.
func (a *AbstractAsset) SetName(name string) *AbstractAsset {
	if a.isIdentifierQuoted(name) {
		a.quoted = true
		name = a.trimQuotes(name)
	}

	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		a.namespace = parts[0]
		name = parts[1]
	}

	a.name = name

	return a
}

// IsInDefaultNamespace - Is this asset in the default namespace?
func (a *AbstractAsset) IsInDefaultNamespace(defaultNamespaceName string) bool {
	return a.namespace == "" || a.namespace == defaultNamespaceName
}

// GetNamespaceName - Gets the namespace name of this asset.
// If NULL is returned this means the default namespace is used.
func (a *AbstractAsset) GetNamespaceName() string {
	return a.namespace
}

// GetShortestName - The shortest name is stripped of the default namespace. All other
// namespaced elements are returned as full-qualified names.
func (a *AbstractAsset) GetShortestName(defaultNamespaceName string) string {
	shortestName := a.GetName()
	if a.namespace == defaultNamespaceName {
		return a.name
	}

	return strings.ToLower(shortestName)
}

// IsQuoted - Checks if this asset's name is quoted.
func (a *AbstractAsset) IsQuoted() bool {
	return a.quoted
}

// isIdentifierQuoted - Checks if this identifier is quoted.
func (a *AbstractAsset) isIdentifierQuoted(identifier string) bool {
	runes := []rune(identifier)

	return len(runes) > 0 && (runes[0] == '`' || runes[0] == '"' || runes[0] == '[')
}

// trimQuotes - Trim quotes from the identifier.
func (a *AbstractAsset) trimQuotes(identifier string) string {
	return strings.NewReplacer(
		"`", "",
		`"`, "",
		"[", "",
		"]", "",
	).Replace(identifier)
}

// GetName - Returns the name of this schema asset.
func (a *AbstractAsset) GetName() string {
	if a.namespace != "" {
		return a.namespace + "." + a.name
	}
	return a.name
}

// GetQuotedName - Gets the quoted representation of this asset but only if it was defined with one. Otherwise
// return the plain unquoted value as inserted.
func (a *AbstractAsset) GetQuotedName(platform AssetsPlatform) string {
	keywords := platform.GetReservedKeywordsList()

	parts := strings.Split(a.name, ".")
	for k, v := range parts {
		if a.IsQuoted() || keywords.IsKeyword(v) {
			parts[k] = platform.QuoteIdentifier(v)
		}
	}

	return strings.Join(parts, ".")
}

// generateIdentifierName - Generates an identifier from a list of column names obeying a certain string length.
// This is especially important for Oracle, since it does not allow identifiers larger than 30 chars,
// however building idents automatically for foreign keys, composite keys or such can easily create
// very long names.
// prefix by default if ""
// maxSize by default 30
func (a *AbstractAsset) generateIdentifierName(
	columnNames []string,
	prefix string,
	maxSize int,
) string {
	hashParts := make([]string, len(columnNames))
	for i, column := range columnNames {
		hashParts[i] = utils.Dechex(int64(crc32.ChecksumIEEE([]byte(column))))
	}

	hash := strings.Join(hashParts, "")

	result := strings.ToUpper(utils.Substr(prefix+"_"+hash, 0, maxSize))

	return result
}
