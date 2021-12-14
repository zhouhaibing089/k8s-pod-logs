package storage

type Interface interface {
	// List returns all object names.
	List() (objects []string, err error)
	// Put uploads the data into storage
	Put(key string, data []byte) error
	// Get retrieves the data at the given key
	Get(key string) (data []byte, err error)
	// Has checks whether the key exists
	Has(key string) (bool, error)
}
