# ConfigLoader

A simple config loader that watches (and polls) a YAML file on disk,
reads and parses it when it changes, and makes the contents available
to subscribers and readers.

The loader is generic; you can provide your own type for the loader
to use. See the unit test for an example.

