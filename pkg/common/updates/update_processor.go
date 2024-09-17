package updates

// type UpdateAction struct {
// 	MachineName string
// 	UpdateData  UpdateData
// 	UpdateFunc  func(*models.Machine, UpdateData)
// }

// type UpdateData struct {
// 	UpdateType    UpdateType
// 	ResourceType  ResourceType
// 	ResourceState string
// 	ServiceType   ServiceType
// 	ServiceState  string
// }

// type UpdateType string
// type ResourceType string
// type ServiceType string

// const (
// 	UpdateTypeComplete UpdateType = "Complete"
// 	UpdateTypeResource UpdateType = "Resource"
// 	UpdateTypeService  UpdateType = "Service"
// )

// type UpdateProcessor struct {
// 	updateQueue         chan UpdateAction
// 	updateMutex         sync.Mutex
// 	updateProcessorDone chan struct{}
// 	getGlobalModelFunc  func() *display.DisplayModel
// }

// func NewUpdateProcessor(getGlobalModelFunc func() *display.DisplayModel) *UpdateProcessor {
// 	return &UpdateProcessor{
// 		updateQueue:         make(chan UpdateAction, 100),
// 		updateProcessorDone: make(chan struct{}),
// 		getGlobalModelFunc:  getGlobalModelFunc,
// 	}
// }

// func (up *UpdateProcessor) StartUpdateProcessor(ctx context.Context) {
// 	l := logger.Get()
// 	up.updateProcessorDone = make(chan struct{})
// 	l.Debug("startUpdateProcessor: Started")
// 	defer close(up.updateProcessorDone)
// 	defer l.Debug("startUpdateProcessor: Finished")

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			l.Debug("startUpdateProcessor: Context cancelled")
// 			return
// 		case update, ok := <-up.updateQueue:
// 			if !ok {
// 				l.Debug("startUpdateProcessor: Update queue closed")
// 				return
// 			}
// 			l.Debug(
// 				fmt.Sprintf(
// 					"startUpdateProcessor: Processing update for %s, %s",
// 					update.MachineName,
// 					update.UpdateData.ResourceType,
// 				),
// 			)
// 			up.ProcessUpdate(update)
// 		}
// 	}
// }

// func (up *UpdateProcessor) ProcessUpdate(update UpdateAction) {
// 	l := logger.Get()
// 	up.updateMutex.Lock()
// 	defer up.updateMutex.Unlock()

// 	m := up.getGlobalModelFunc()
// 	if m == nil || m.Deployment == nil || m.Deployment.Machines == nil {
// 		l.Debug("processUpdate: Global model, deployment, or machines is nil")
// 		return
// 	}

// 	machine, ok := m.Deployment.Machines[update.MachineName]
// 	if !ok {
// 		l.Debug(fmt.Sprintf("processUpdate: Machine %s not found", update.MachineName))
// 		return
// 	}

// 	if update.UpdateFunc == nil {
// 		l.Error("processUpdate: UpdateFunc is nil")
// 		return
// 	}

// 	switch update.UpdateData.UpdateType {
// 	case UpdateTypeComplete:
// 		machine.SetComplete()
// 	case UpdateTypeResource:
// 		machine.SetMachineResourceState(
// 			string(update.UpdateData.ResourceType),
// 			models.MachineResourceState(update.UpdateData.ResourceState),
// 		)
// 	case UpdateTypeService:
// 		machine.SetServiceState(
// 			string(update.UpdateData.ServiceType),
// 			models.ServiceState(update.UpdateData.ServiceState),
// 		)
// 	default:
// 		l.Errorf("processUpdate: Unknown UpdateType %s", update.UpdateData.UpdateType)
// 	}

// 	update.UpdateFunc(machine, update.UpdateData)
// }

// func (up *UpdateProcessor) QueueUpdate(update UpdateAction) {
// 	up.updateQueue <- update
// }

// func (up *UpdateProcessor) Stop() {
// 	close(up.updateQueue)
// 	<-up.updateProcessorDone
// }
