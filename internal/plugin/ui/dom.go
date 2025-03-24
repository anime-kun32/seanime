package plugin_ui

import (
	"seanime/internal/util/result"

	"github.com/dop251/goja"
	"github.com/google/uuid"
)

// DOMManager handles DOM manipulation requests from plugins
type DOMManager struct {
	ctx              *Context
	elementObservers *result.Map[string, *ElementObserver]
	eventListeners   *result.Map[string, *DOMEventListener]
}

type ElementObserver struct {
	ID       string
	Selector string
	Callback goja.Callable
}

type DOMEventListener struct {
	ID        string
	ElementID string
	EventType string
	Callback  goja.Callable
}

// NewDOMManager creates a new DOM manager
func NewDOMManager(ctx *Context) *DOMManager {
	return &DOMManager{
		ctx:              ctx,
		elementObservers: result.NewResultMap[string, *ElementObserver](),
		eventListeners:   result.NewResultMap[string, *DOMEventListener](),
	}
}

// BindToObj binds DOM manipulation methods to a context object
func (d *DOMManager) BindToObj(vm *goja.Runtime, obj *goja.Object) {
	domObj := vm.NewObject()
	_ = domObj.Set("query", d.jsQuery)
	_ = domObj.Set("queryOne", d.jsQueryOne)
	_ = domObj.Set("observe", d.jsObserve)
	_ = domObj.Set("createElement", d.jsCreateElement)
	_ = domObj.Set("onReady", d.jsOnReady)

	_ = obj.Set("dom", domObj)
}

func (d *DOMManager) jsOnReady(call goja.FunctionCall) goja.Value {

	callback, ok := goja.AssertFunction(call.Argument(0))
	if !ok {
		d.ctx.handleTypeError("onReady requires a callback function")
	}

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMReadyEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		d.ctx.scheduler.ScheduleAsync(func() error {
			_, err := callback(goja.Undefined(), d.ctx.vm.ToValue(event.Payload))
			if err != nil {
				d.ctx.handleException(err)
			}
			return nil
		})
		d.ctx.UnregisterEventListener(listener.ID)
	})

	return d.ctx.vm.ToValue(nil)
}

// jsQuery handles querying for multiple DOM elements
func (d *DOMManager) jsQuery(call goja.FunctionCall) goja.Value {
	selector := call.Argument(0).String()

	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Set up a one-time event listener for the response
	listener := d.ctx.RegisterEventListener(ClientDOMQueryResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMQueryResultEventPayload
		if event.ParsePayloadAs(ClientDOMQueryResultEvent, &payload) && payload.RequestID == requestId {
			d.ctx.scheduler.ScheduleAsync(func() error {
				elemObjs := make([]interface{}, 0, len(payload.Elements))
				for _, elem := range payload.Elements {
					if elemData, ok := elem.(map[string]interface{}); ok {
						elemObjs = append(elemObjs, d.createDOMElementObject(elemData))
					}
				}
				resolve(d.ctx.vm.ToValue(elemObjs))
				return nil
			})
			d.ctx.UnregisterEventListener(listener.ID)
		}
	})

	// Send the query request to the client
	d.ctx.SendEventToClient(ServerDOMQueryEvent, &ServerDOMQueryEventPayload{
		Selector:  selector,
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

// jsQueryOne handles querying for a single DOM element
func (d *DOMManager) jsQueryOne(call goja.FunctionCall) goja.Value {
	selector := call.Argument(0).String()

	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Set up a one-time event listener for the response
	listener := d.ctx.RegisterEventListener(ClientDOMQueryOneResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMQueryOneResultEventPayload
		if event.ParsePayloadAs(ClientDOMQueryOneResultEvent, &payload) && payload.RequestID == requestId {
			d.ctx.scheduler.ScheduleAsync(func() error {
				if payload.Element != nil {
					if elemData, ok := payload.Element.(map[string]interface{}); ok {
						resolve(d.ctx.vm.ToValue(d.createDOMElementObject(elemData)))
					} else {
						resolve(goja.Null())
					}
				} else {
					resolve(goja.Null())
				}
				return nil
			})
			d.ctx.UnregisterEventListener(listener.ID)
		}
	})

	// Send the query request to the client
	d.ctx.SendEventToClient(ServerDOMQueryOneEvent, &ServerDOMQueryOneEventPayload{
		Selector:  selector,
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

// jsObserve starts observing DOM elements matching a selector
func (d *DOMManager) jsObserve(call goja.FunctionCall) goja.Value {
	selector := call.Argument(0).String()
	callback, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		d.ctx.handleTypeError("observe requires a callback function")
	}

	// Create observer ID
	observerID := uuid.New().String()

	// Store the observer
	observer := &ElementObserver{
		ID:       observerID,
		Selector: selector,
		Callback: callback,
	}

	d.elementObservers.Set(observerID, observer)

	// Send observe request to client
	d.ctx.SendEventToClient(ServerDOMObserveEvent, &ServerDOMObserveEventPayload{
		Selector:   selector,
		ObserverID: observerID,
	})

	// Start a goroutine to handle observer updates
	listener := d.ctx.RegisterEventListener(ClientDOMObserveResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMObserveResultEventPayload
		if event.ParsePayloadAs(ClientDOMObserveResultEvent, &payload) && payload.ObserverID == observerID {
			d.ctx.scheduler.ScheduleAsync(func() error {
				observer, exists := d.elementObservers.Get(observerID)

				if !exists {
					return nil
				}

				// Convert elements to DOM element objects directly in the VM thread
				elemObjs := make([]interface{}, 0, len(payload.Elements))
				for _, elem := range payload.Elements {
					if elemData, ok := elem.(map[string]interface{}); ok {
						elemObjs = append(elemObjs, d.createDOMElementObject(elemData))
					}
				}

				// Call the callback directly now that we have all elements
				_, err := observer.Callback(goja.Undefined(), d.ctx.vm.ToValue(elemObjs))
				if err != nil {
					d.ctx.handleException(err)
				}
				return nil
			})
		}
	})

	// Listen for DOM ready events to re-observe elements after page reload
	domReadyListener := d.ctx.RegisterEventListener(ClientDOMReadyEvent)

	domReadyListener.SetCallback(func(event *ClientPluginEvent) {
		// Re-send the observe request when the DOM is ready
		d.ctx.SendEventToClient(ServerDOMObserveEvent, &ServerDOMObserveEventPayload{
			Selector:   selector,
			ObserverID: observerID,
		})
		d.ctx.UnregisterEventListener(domReadyListener.ID)
	})

	// Return a function to stop observing
	cancelFn := func() {
		d.ctx.UnregisterEventListener(listener.ID)
		d.ctx.UnregisterEventListener(domReadyListener.ID)
		d.elementObservers.Delete(observerID)

		d.ctx.SendEventToClient(ServerDOMStopObserveEvent, &ServerDOMStopObserveEventPayload{
			ObserverID: observerID,
		})
	}

	refetchFn := func() {
		d.ctx.SendEventToClient(ServerDOMObserveEvent, &ServerDOMObserveEventPayload{
			Selector:   selector,
			ObserverID: observerID,
		})
	}

	d.ctx.registerOnCleanup(func() {
		cancelFn()
	})

	return d.ctx.vm.ToValue([]interface{}{cancelFn, refetchFn})
}

// jsCreateElement creates a new DOM element
func (d *DOMManager) jsCreateElement(call goja.FunctionCall) goja.Value {
	tagName := call.Argument(0).String()

	// Create a promise that will be resolved with the created element
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Set up a one-time event listener for the response
	listener := d.ctx.RegisterEventListener(ClientDOMCreateResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMCreateResultEventPayload
		if event.ParsePayloadAs(ClientDOMCreateResultEvent, &payload) && payload.RequestID == requestId {
			if elemData, ok := payload.Element.(map[string]interface{}); ok {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.createDOMElementObject(elemData))
					return nil
				})
			}
			d.ctx.UnregisterEventListener(listener.ID)
		}
	})

	// Send the create request to the client
	d.ctx.SendEventToClient(ServerDOMCreateEvent, &ServerDOMCreateEventPayload{
		TagName:   tagName,
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

// HandleObserverUpdate processes DOM observer updates from client
func (d *DOMManager) HandleObserverUpdate(observerID string, elements []interface{}) {

}

// HandleDOMEvent processes DOM events from client
func (d *DOMManager) HandleDOMEvent(elementId string, eventType string, eventData map[string]interface{}) {
	// Find all event listeners for this element and event type
	d.eventListeners.Range(func(key string, listener *DOMEventListener) bool {
		if listener.ElementID == elementId && listener.EventType == eventType {
			// Schedule callback execution in the VM
			d.ctx.scheduler.ScheduleAsync(func() error {
				_, err := listener.Callback(goja.Undefined(), d.ctx.vm.ToValue(eventData))
				if err != nil {
					d.ctx.handleException(err)
				}
				return nil
			})
		}
		return true
	})
}

// createDOMElementObject creates a JavaScript object representing a DOM element
func (d *DOMManager) createDOMElementObject(elemData map[string]interface{}) *goja.Object {
	elementObj := d.ctx.vm.NewObject()

	// Set basic properties
	elementId, _ := elemData["id"].(string)
	_ = elementObj.Set("id", elementId)

	if tagName, ok := elemData["tagName"].(string); ok {
		_ = elementObj.Set("tagName", tagName)
	}

	if text, ok := elemData["text"].(string); ok {
		_ = elementObj.Set("text", text)
	}

	if attributes, ok := elemData["attributes"].(map[string]interface{}); ok {
		for key, value := range attributes {
			_ = elementObj.Set(key, value)
		}
	}

	if style, ok := elemData["style"].(map[string]interface{}); ok {
		for key, value := range style {
			_ = elementObj.Set(key, value)
		}
	}

	if className, ok := elemData["className"].(string); ok {
		_ = elementObj.Set("className", className)
	}

	if classList, ok := elemData["classList"].([]string); ok {
		_ = elementObj.Set("classList", classList)
	}

	if children, ok := elemData["children"].([]interface{}); ok {
		childrenObjs := make([]*goja.Object, 0, len(children))
		for _, child := range children {
			if childData, ok := child.(map[string]interface{}); ok {
				childrenObjs = append(childrenObjs, d.createDOMElementObject(childData))
			}
		}
		_ = elementObj.Set("children", childrenObjs)
	}

	if parent, ok := elemData["parent"].(map[string]interface{}); ok {
		elementObj.Set("parent", d.createDOMElementObject(parent))
	}

	// Define methods
	_ = elementObj.Set("getText", func() goja.Value {
		return d.getElementText(elementId)
	})

	_ = elementObj.Set("setText", func(text string) {
		d.setElementText(elementId, text)
	})

	_ = elementObj.Set("getAttribute", func(name string) goja.Value {
		return d.getElementAttribute(elementId, name)
	})

	_ = elementObj.Set("getAttributes", func() goja.Value {
		return d.getElementAttributes(elementId)
	})

	_ = elementObj.Set("setAttribute", func(name, value string) {
		d.setElementAttribute(elementId, name, value)
	})

	_ = elementObj.Set("removeAttribute", func(name string) {
		d.removeElementAttribute(elementId, name)
	})

	_ = elementObj.Set("hasAttribute", func(name string) goja.Value {
		return d.hasElementAttribute(elementId, name)
	})

	_ = elementObj.Set("getProperty", func(name string) goja.Value {
		return d.getElementProperty(elementId, name)
	})

	_ = elementObj.Set("setProperty", func(name string, value interface{}) {
		d.setElementProperty(elementId, name, value)
	})

	_ = elementObj.Set("addClass", func(className string) {
		d.addElementClass(elementId, className)
	})

	_ = elementObj.Set("removeClass", func(className string) {
		d.removeElementClass(elementId, className)
	})

	_ = elementObj.Set("hasClass", func(className string) goja.Value {
		return d.hasElementClass(elementId, className)
	})

	_ = elementObj.Set("setStyle", func(property, value string) {
		d.setElementStyle(elementId, property, value)
	})

	_ = elementObj.Set("getStyle", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) {
			property := call.Argument(0).String()
			return d.ctx.vm.ToValue(d.getElementStyle(elementId, property))
		}
		return d.ctx.vm.ToValue(d.getElementStyles(elementId))
	})

	_ = elementObj.Set("getComputedStyle", func(property string) goja.Value {
		return d.getElementComputedStyle(elementId, property)
	})

	_ = elementObj.Set("append", func(child *goja.Object) {
		childID := child.Get("id").String()
		d.appendElement(elementId, childID)
	})

	_ = elementObj.Set("before", func(sibling *goja.Object) {
		siblingID := sibling.Get("id").String()
		d.insertElementBefore(elementId, siblingID)
	})

	_ = elementObj.Set("after", func(sibling *goja.Object) {
		siblingID := sibling.Get("id").String()
		d.insertElementAfter(elementId, siblingID)
	})

	_ = elementObj.Set("remove", func() {
		d.removeElement(elementId)
	})

	_ = elementObj.Set("getParent", func() goja.Value {
		return d.getElementParent(elementId)
	})

	_ = elementObj.Set("getChildren", func() goja.Value {
		return d.getElementChildren(elementId)
	})

	_ = elementObj.Set("addEventListener", func(event string, callback goja.Callable) func() {
		return d.addElementEventListener(elementId, event, callback)
	})

	_ = elementObj.Set("getDataAttribute", func(key string) goja.Value {
		return d.getElementDataAttribute(elementId, key)
	})

	_ = elementObj.Set("getDataAttributes", func() goja.Value {
		return d.getElementDataAttributes(elementId)
	})

	_ = elementObj.Set("setDataAttribute", func(key, value string) {
		d.setElementDataAttribute(elementId, key, value)
	})

	_ = elementObj.Set("removeDataAttribute", func(key string) {
		d.removeElementDataAttribute(elementId, key)
	})

	_ = elementObj.Set("hasDataAttribute", func(key string) goja.Value {
		return d.hasElementDataAttribute(elementId, key)
	})

	_ = elementObj.Set("hasStyle", func(property string) goja.Value {
		return d.hasElementStyle(elementId, property)
	})

	_ = elementObj.Set("removeStyle", func(property string) {
		d.removeElementStyle(elementId, property)
	})

	// Add element query methods
	_ = elementObj.Set("query", func(selector string) goja.Value {
		return d.elementQuery(elementId, selector)
	})

	_ = elementObj.Set("queryOne", func(selector string) goja.Value {
		return d.elementQueryOne(elementId, selector)
	})

	return elementObj
}

// Element manipulation methods
// These send events to the client and handle responses

func (d *DOMManager) getElementText(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			// Only process responses with matching element ID, action, and request ID
			if payload.Action == "getText" && payload.ElementID == elementId && payload.RequestID == requestId {
				if v, ok := payload.Result.(string); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(""))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	// Send the request to the client with the request ID
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getText",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) setElementText(elementId, text string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "setText",
		Params: map[string]interface{}{
			"text": text,
		},
	})
}

func (d *DOMManager) getElementAttribute(elementId, name string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			// Only process responses with matching element ID, action, and request ID
			if payload.Action == "getAttribute" && payload.ElementID == elementId && payload.RequestID == requestId {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getAttribute",
		Params: map[string]interface{}{
			"name": name,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) setElementAttribute(elementId, name, value string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "setAttribute",
		Params: map[string]interface{}{
			"name":  name,
			"value": value,
		},
	})
}

func (d *DOMManager) removeElementAttribute(elementId, name string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "removeAttribute",
		Params: map[string]interface{}{
			"name": name,
		},
	})
}

func (d *DOMManager) addElementClass(elementId, className string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "addClass",
		Params: map[string]interface{}{
			"className": className,
		},
	})
}

func (d *DOMManager) removeElementClass(elementId, className string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "removeClass",
		Params: map[string]interface{}{
			"className": className,
		},
	})
}

func (d *DOMManager) hasElementClass(elementId, className string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			// Only process responses with matching element ID, action, and request ID
			if payload.Action == "hasClass" && payload.ElementID == elementId && payload.RequestID == requestId {
				if v, ok := payload.Result.(bool); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(false))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "hasClass",
		Params: map[string]interface{}{
			"className": className,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) setElementStyle(elementId, property, value string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "setStyle",
		Params: map[string]interface{}{
			"property": property,
			"value":    value,
		},
	})
}

func (d *DOMManager) getElementStyle(elementId, property string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) && payload.ElementID == elementId {
			if payload.Action == "getStyle" && payload.RequestID == requestId {
				if v, ok := payload.Result.(string); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(""))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getStyle",
		Params: map[string]interface{}{
			"property": property,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) getElementComputedStyle(elementId, property string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) && payload.ElementID == elementId {
			if payload.Action == "getComputedStyle" && payload.RequestID == requestId {
				if v, ok := payload.Result.(string); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(""))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getComputedStyle",
		Params: map[string]interface{}{
			"property": property,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) appendElement(parentID, childID string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: parentID,
		Action:    "append",
		Params: map[string]interface{}{
			"childID": childID,
		},
	})
}

func (d *DOMManager) insertElementBefore(elementId, siblingID string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "before",
		Params: map[string]interface{}{
			"siblingID": siblingID,
		},
	})
}

func (d *DOMManager) insertElementAfter(elementId, siblingID string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "after",
		Params: map[string]interface{}{
			"siblingID": siblingID,
		},
	})
}

func (d *DOMManager) removeElement(elementId string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "remove",
		Params:    map[string]interface{}{},
	})
}

func (d *DOMManager) getElementParent(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getParent" && payload.ElementID == elementId && payload.RequestID == requestId {
				if payload.Result != nil {
					if parentData, ok := payload.Result.(map[string]interface{}); ok {
						d.ctx.scheduler.ScheduleAsync(func() error {
							resolve(d.ctx.vm.ToValue(d.createDOMElementObject(parentData)))
							return nil
						})
					} else {
						d.ctx.scheduler.ScheduleAsync(func() error {
							resolve(goja.Null())
							return nil
						})
					}
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(goja.Null())
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getParent",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) getElementChildren(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {

			if payload.Action == "getChildren" && payload.ElementID == elementId && payload.RequestID == requestId {
				if payload.Result != nil {
					if childrenData, ok := payload.Result.([]interface{}); ok {
						d.ctx.scheduler.ScheduleAsync(func() error {
							childrenObjs := make([]interface{}, 0, len(childrenData))
							for _, child := range childrenData {
								if childData, ok := child.(map[string]interface{}); ok {
									childrenObjs = append(childrenObjs, d.createDOMElementObject(childData))
								}
							}
							resolve(d.ctx.vm.ToValue(childrenObjs))
							return nil
						})
					} else {
						d.ctx.scheduler.ScheduleAsync(func() error {
							resolve(d.ctx.vm.ToValue([]interface{}{}))
							return nil
						})
					}
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue([]interface{}{}))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getChildren",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) addElementEventListener(elementId, event string, callback goja.Callable) func() {
	// Create a unique ID for this event listener
	listenerID := uuid.New().String()

	// Store the event listener
	listener := &DOMEventListener{
		ID:        listenerID,
		ElementID: elementId,
		EventType: event,
		Callback:  callback,
	}

	d.eventListeners.Set(listenerID, listener)

	// Send the request to add the event listener to the client
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "addEventListener",
		Params: map[string]interface{}{
			"event":      event,
			"listenerID": listenerID,
		},
	})

	// Return a function to remove the event listener
	return func() {
		d.eventListeners.Delete(listenerID)

		// Send the request to remove the event listener from the client
		d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
			ElementID: elementId,
			Action:    "removeEventListener",
			Params: map[string]interface{}{
				"event":      event,
				"listenerID": listenerID,
			},
		})
	}
}

func (d *DOMManager) getElementAttributes(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getAttributes" && payload.ElementID == elementId && payload.RequestID == requestId {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getAttributes",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) hasElementAttribute(elementId, name string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "hasAttribute" && payload.ElementID == elementId && payload.RequestID == requestId {
				if v, ok := payload.Result.(bool); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(false))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "hasAttribute",
		Params: map[string]interface{}{
			"name": name,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) getElementProperty(elementId, name string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getProperty" && payload.ElementID == elementId && payload.RequestID == requestId {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getProperty",
		Params: map[string]interface{}{
			"name": name,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) setElementProperty(elementId, name string, value interface{}) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "setProperty",
		Params: map[string]interface{}{
			"name":  name,
			"value": value,
		},
	})
}

func (d *DOMManager) getElementStyles(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getStyle" && payload.ElementID == elementId && payload.RequestID == requestId && payload.Result != nil {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getStyle",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) hasElementStyle(elementId, property string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "hasStyle" && payload.ElementID == elementId && payload.RequestID == requestId {
				if v, ok := payload.Result.(bool); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(false))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "hasStyle",
		Params: map[string]interface{}{
			"property": property,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) getElementDataAttribute(elementId, key string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getDataAttribute" && payload.ElementID == elementId && payload.RequestID == requestId {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getDataAttribute",
		Params: map[string]interface{}{
			"key": key,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) getElementDataAttributes(elementId string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "getDataAttributes" && payload.ElementID == elementId && payload.RequestID == requestId {
				d.ctx.scheduler.ScheduleAsync(func() error {
					resolve(d.ctx.vm.ToValue(payload.Result))
					return nil
				})
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "getDataAttributes",
		Params:    map[string]interface{}{},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) setElementDataAttribute(elementId, key, value string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "setDataAttribute",
		Params: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	})
}

func (d *DOMManager) removeElementDataAttribute(elementId, key string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "removeDataAttribute",
		Params: map[string]interface{}{
			"key": key,
		},
	})
}

func (d *DOMManager) hasElementDataAttribute(elementId, key string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Listen for changes from the client
	listener := d.ctx.RegisterEventListener(ClientDOMElementUpdatedEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMElementUpdatedEventPayload
		if event.ParsePayloadAs(ClientDOMElementUpdatedEvent, &payload) {
			if payload.Action == "hasDataAttribute" && payload.ElementID == elementId && payload.RequestID == requestId {
				if v, ok := payload.Result.(bool); ok {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(v))
						return nil
					})
				} else {
					d.ctx.scheduler.ScheduleAsync(func() error {
						resolve(d.ctx.vm.ToValue(false))
						return nil
					})
				}
				d.ctx.UnregisterEventListener(listener.ID)
			}
		}
	})

	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "hasDataAttribute",
		Params: map[string]interface{}{
			"key": key,
		},
		RequestID: requestId,
	})

	return d.ctx.vm.ToValue(promise)
}

func (d *DOMManager) removeElementStyle(elementId, property string) {
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "removeStyle",
		Params: map[string]interface{}{
			"property": property,
		},
	})
}

// elementQuery handles querying for multiple DOM elements from a parent element
func (d *DOMManager) elementQuery(elementId, selector string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Set up a one-time event listener for the response
	listener := d.ctx.RegisterEventListener(ClientDOMQueryResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMQueryResultEventPayload
		if event.ParsePayloadAs(ClientDOMQueryResultEvent, &payload) && payload.RequestID == requestId {
			d.ctx.scheduler.ScheduleAsync(func() error {
				elemObjs := make([]interface{}, 0, len(payload.Elements))
				for _, elem := range payload.Elements {
					if elemData, ok := elem.(map[string]interface{}); ok {
						elemObjs = append(elemObjs, d.createDOMElementObject(elemData))
					}
				}
				resolve(d.ctx.vm.ToValue(elemObjs))
				return nil
			})
			d.ctx.UnregisterEventListener(listener.ID)
		}
	})

	// Send the query request to the client
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "query",
		Params: map[string]interface{}{
			"selector":  selector,
			"requestId": requestId,
		},
	})

	return d.ctx.vm.ToValue(promise)
}

// elementQueryOne handles querying for a single DOM element from a parent element
func (d *DOMManager) elementQueryOne(elementId, selector string) goja.Value {
	promise, resolve, _ := d.ctx.vm.NewPromise()

	// Generate a unique request ID
	requestId := uuid.New().String()

	// Set up a one-time event listener for the response
	listener := d.ctx.RegisterEventListener(ClientDOMQueryOneResultEvent)

	listener.SetCallback(func(event *ClientPluginEvent) {
		var payload ClientDOMQueryOneResultEventPayload
		if event.ParsePayloadAs(ClientDOMQueryOneResultEvent, &payload) && payload.RequestID == requestId {
			d.ctx.scheduler.ScheduleAsync(func() error {
				if payload.Element != nil {
					if elemData, ok := payload.Element.(map[string]interface{}); ok {
						resolve(d.ctx.vm.ToValue(d.createDOMElementObject(elemData)))
					} else {
						resolve(goja.Null())
					}
				} else {
					resolve(goja.Null())
				}
				return nil
			})
			d.ctx.UnregisterEventListener(listener.ID)
		}
	})

	// Send the query request to the client
	d.ctx.SendEventToClient(ServerDOMManipulateEvent, &ServerDOMManipulateEventPayload{
		ElementID: elementId,
		Action:    "queryOne",
		Params: map[string]interface{}{
			"selector":  selector,
			"requestId": requestId,
		},
	})

	return d.ctx.vm.ToValue(promise)
}
