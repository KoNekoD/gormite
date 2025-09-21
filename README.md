# Gormite

Migrations generator and query builder with mapping toolset

## Install and configure the migration generator
### Step 1: Install Gormite

`go install github.com/KoNekoD/gormite/cmd/gormite@latest`

### Step 2 Create a configuration file gormite.yaml

Gormite looks for default settings in the resources directory.

**resources/gormite.yaml**

```yaml
gormite:  
  orm:    
    mapping:      
      Entities:
        dir: pkg/entities
```
    dir - specifies where your Go entities are stored.

### Step 3: Prepare the entities

Inside pkg/entities, create files with structures describing the database tables.

**user.go**

```go
package entities

import “github.com/KoNekoD/gormite/test/docs_example/pkg/enums”.

// User “app_user”
type User struct {	
  ID int `db: “id” pk: “true”`	
  Email string `db: “email” uniq: “email” length: “180” uniq_cond: “email:(identity_type = ‘email’)”`	
  Phone string `db: “phone” uniq: “phone” length: “10” uniq_cond: “phone:(identity_type = ‘phone’)”`	
  IdentityType enums.IdentityType `db: “identity_type” type: “varchar”`	
  Code *string `db: “code” nullable: “true”`	
  FullName *string `db: “full_name” default:“‘Anonymous’” index: “index_full_name” index_cond: “index_full_name:(is_active = true)”`	
  IsActive bool `db: “is_active”`
}

type UserProfile struct {	
  ID int `db: “id” pk: “true”`	
  User *User `db: “user_id”`	
  Age int `db: “age”`
}
```

Once you have prepared the entities and YAML file, Gormite analyzes the structures and generates SQL migrations, taking everything into account

