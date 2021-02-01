package apierrors

import (
	"fmt"
	"testing"
)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"Test Transient capitalised error",
			fmt.Errorf("Error updating the status of the MongoDB resource: Operation cannot be fulfilled on mongodbcommunity.mongodb.com \"mdb0\": The object has been modified; please apply your changes to the latest version and try again"),
			true,
		},
		{
			"Test Transient lower case error",
			fmt.Errorf("error updating the status of the MongoDB resource: Operation cannot be fulfilled on mongodbcommunity.mongodb.com \"mdb0\": the object has been modified; please apply your changes to the latest version and try again"),
			true,
		},
		{
			"Test Not Transient Errir",
			fmt.Errorf(" error found deployments.extensions \"default\" not found"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransient(tt.err); got != tt.want {
				t.Errorf("IsTransient() = %v, want %v", got, tt.want)
			}
		})
	}
}
