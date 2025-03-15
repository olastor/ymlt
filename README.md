# ymlt

`ymlt` is a command-line tool for processing YAML documents with support for default values.

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

Apply defaults to a YAML file:

```shell
ymlt -d defaults.yaml input.yaml
```

Read YAML from stdin and apply defaults:

```shell
cat input.yaml | ymlt -d defaults.yaml
```

