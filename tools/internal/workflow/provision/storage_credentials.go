package provision

type StorageCredentials = storageCredentials

func LoadStorageCredentials(clusterName string) (credentials StorageCredentials, ok bool, err error) {
	return loadStorageCredentials(clusterName)
}
