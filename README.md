# ymlt

`ymlt` is a command-line tool for processing YAML documents, allowing the use of Go templates in every string field, with added functions `t` and `tt` to lookup values from within the same document.

## Usage

```shell
ymlt [options] [file]
```

## Options

- `-d, --defaults string`  Set default values from a YAML file
- `-v` Display version
- `-h, --help` Display help

## Installation

You can build `ymlt` from source:

```shell
make build
```

## Examples

### Basic Template Usage

`ymlt` allows you to use Go templates in YAML string fields with special functions `t()` and `tt()` to reference other values in the same document:

**Input file (`data.yaml`):**
```yaml
name: John
age: 32
ageCopy: "{{ t \".age\" }}"
nameCopy: '{{ t "$.name" }}'
---
name: Peter
age: 98
nameAndAge: |
  {{ t ".name" }} ({{ t "$.age" }} years old)
```

**Command:**
```shell
ymlt data.yaml
```

**Output:**
```yaml
name: John
age: 32
ageCopy: "32"
nameCopy: 'John'
---
name: Peter
age: 98
nameAndAge: |
  Peter (98 years old)
```

### Using Defaults

You can provide default values that will be merged into your YAML document. The defaults can also use template variables to reference fields from the main document:

**Input file (`data.yaml`):**
```yaml
name: John
age: 32
---
name: Jelena
age: 59
address: Test Street 4
```

**Defaults file (`defaults.yaml`):**
```yaml
address: unknown
description: |
  {{ t ".name" }} is {{ t ".age" }} years old
```

**Command:**
```shell
ymlt -d defaults.yaml data.yaml
```

**Output:**
```yaml
name: John
age: 32
address: unknown
description: |
  John is 32 years old
---
name: Jelena
age: 59
address: Test Street 4
description: |
  Jelena is 59 years old
```

### Complex Template Example

Here's a more complex example showing how to build formatted strings using multiple field references:

**Input file (`addresses.yaml`):**
```yaml
street: "Main Street"
houseNumber: "42"
postalCode: "90210"
country: "USA"
---
street: "Elm Street"
houseNumber: "13A"
postalCode: "10001"
country: "USA"
```

**Defaults file (`address-defaults.yaml`):**
```yaml
address: |
  {{ t ".street" }} {{ t ".houseNumber"}},
  {{ t ".postalCode" }} ({{ t ".country" }})
```

**Command:**
```shell
ymlt -d address-defaults.yaml addresses.yaml
```

**Output:**
```yaml
street: "Main Street"
houseNumber: "42"
postalCode: "90210"
country: "USA"
address: |
  Main Street 42,
  90210 (USA)
---
street: "Elm Street"
houseNumber: "13A"
postalCode: "10001"
country: "USA"
address: |
  Elm Street 13A,
  10001 (USA)
```

### Reading from stdin

You can also process YAML from stdin:

```shell
cat input.yaml | ymlt -d defaults.yaml
```

