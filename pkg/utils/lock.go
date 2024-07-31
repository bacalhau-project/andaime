package utils

import (
	"sync"

	"github.com/bacalhau-project/andaime/pkg/models"
)

var (
	StatusMutex     sync.RWMutex
	GlobalStatusMap = make(map[string]*models.Status)
)
