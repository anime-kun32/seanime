package plugin_ui

import (
	"seanime/internal/events"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/rs/zerolog"
)

const (
	MAX_EXCEPTIONS                 = 5               // Maximum number of exceptions that can be thrown before the UI is interrupted
	MAX_EFFECT_CALLBACKS           = 100             // Maximum number of effects that can be scheduled before the UI is interrupted
	RESET_EFFECT_CALLBACK_INTERVAL = 1 * time.Second // After this interval, the UI will reset the effect callstack
	MAX_CONCURRENT_FETCH_REQUESTS  = 10              // Maximum number of concurrent fetch requests
)

// UI registry, unique to a plugin and VM
type UI struct {
	extensionID    string
	context        *Context
	mu             sync.RWMutex
	vm             *goja.Runtime // VM executing the UI
	logger         *zerolog.Logger
	wsEventManager events.WSEventManagerInterface
}

func (u *UI) ClearInterrupt() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.vm.ClearInterrupt()
	u.context.scheduler.Stop()
	if u.context.wsSubscriber != nil {
		u.wsEventManager.UnsubscribeFromClientEvents("plugin-" + u.extensionID)
	}
}

func NewUI(extensionID string, logger *zerolog.Logger, vm *goja.Runtime, wsEventManager events.WSEventManagerInterface) *UI {
	mLogger := logger.With().Str("id", extensionID).Logger()
	ui := &UI{
		extensionID:    extensionID,
		vm:             vm,
		logger:         &mLogger,
		wsEventManager: wsEventManager,
	}
	ui.context = NewContext(ui, extensionID, &mLogger, vm, wsEventManager)
	return ui
}

// Register a UI
// This is the main entry point for the UI
// - It is called once when the plugin is loaded and registers all necessary modules
func (u *UI) Register(callback string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Create a wrapper JavaScript function that calls the provided callback
	callback = `function(ctx) { return (` + callback + `).call(undefined, ctx); }`
	// Compile the callback into a Goja program
	// pr := goja.MustCompile("", "{("+callback+").apply(undefined, __ctx)}", true)

	// Subscribe the plugin to client events
	u.context.wsSubscriber = u.wsEventManager.SubscribeToClientEvents("plugin-" + u.extensionID)

	// Listen for client events and send them to the event listeners
	go func() {
		for event := range u.context.wsSubscriber.Channel {
			//u.logger.Trace().Msgf("Received event %s", event.Type)
			if event.Type == events.PluginEvent {

				if payload, ok := event.Payload.(map[string]interface{}); ok {
					clientEvent := NewClientPluginEvent(payload)
					// If the extension ID is not set, or the extension ID is the same as the current plugin, send the event to the listeners
					if clientEvent.ExtensionID == "" || clientEvent.ExtensionID == u.extensionID {

						switch clientEvent.Type {
						case ClientRenderTraysEvent: // Client wants to render the trays
							u.context.trayManager.renderTray()
						case ClientRenderTrayEvent: // Client wants to render the screen
							u.context.trayManager.renderTray()
						default:
							u.context.eventListeners.Range(func(key string, listener *EventListener) bool {
								//util.SpewMany("Event to listeners", event.Payload)
								if len(listener.ListenTo) > 0 {
									// Check if the event type is in the listener's list of event types
									for _, eventType := range listener.ListenTo {
										if eventType == clientEvent.Type {
											listener.Channel <- clientEvent // Only send the event to the listener if the event type is in the list
										}
									}
								} else {
									listener.Channel <- clientEvent
								}
								return true
							})
						}
					}

				}
			}
		}
		u.logger.Warn().Msg("plugin: Unsubscribed from client events")
		// Close the listener channels when the subscriber is closed
		u.context.eventListeners.Range(func(key string, listener *EventListener) bool {
			close(listener.Channel)
			return true
		})
	}()

	contextObj := u.vm.NewObject()

	_ = contextObj.Set("newTray", u.context.trayManager.jsNewTray)
	_ = contextObj.Set("newForm", u.context.formManager.jsNewForm)

	_ = contextObj.Set("state", u.context.jsState)
	_ = contextObj.Set("setTimeout", u.context.jsSetTimeout)
	_ = contextObj.Set("sleep", u.context.jsSleep)
	_ = contextObj.Set("setInterval", u.context.jsSetInterval)
	_ = contextObj.Set("effect", u.context.jsEffect)
	_ = contextObj.Set("fetch", func(call goja.FunctionCall) goja.Value {
		return u.vm.ToValue(u.context.jsFetch(call))
	})

	_ = u.vm.Set("__ctx", contextObj)

	// Webview (UNUSED)
	webviewObj := u.vm.NewObject()
	_ = contextObj.Set("webview", webviewObj)

	// Screen
	u.context.screenManager.bind(u.vm, contextObj)

	// Execute the callback
	_, err := u.vm.RunString(`(` + callback + `).call(undefined, __ctx)`)
	if err != nil {
		u.logger.Error().Err(err).Msg("plugin: Failed to register UI")
		return
	}
}
