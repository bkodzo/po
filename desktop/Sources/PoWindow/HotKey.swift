import Carbon.HIToolbox

/// A system-wide keyboard shortcut. It works while any app is in front, and
/// needs no special permission, unlike watching the keyboard.
final class HotKey {
    private static var current: HotKey?
    private var handlerRef: EventHandlerRef?
    private var keyRef: EventHotKeyRef?
    private let onPress: () -> Void

    /// Returns nil if the shortcut could not be registered, for instance because
    /// another app already uses it.
    init?(keyCode: Int, modifiers: Int, onPress: @escaping () -> Void) {
        self.onPress = onPress
        var spec = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        let installed = InstallEventHandler(GetApplicationEventTarget(), { _, _, _ in
            HotKey.current?.onPress()
            return noErr
        }, 1, &spec, nil, &handlerRef)
        guard installed == noErr else { return nil }
        let id = EventHotKeyID(signature: OSType(0x506F5F31), id: 1) // "Po_1"
        let registered = RegisterEventHotKey(UInt32(keyCode), UInt32(modifiers), id, GetApplicationEventTarget(), 0, &keyRef)
        guard registered == noErr else { return nil }
        HotKey.current = self
    }

    deinit {
        if let keyRef = keyRef { UnregisterEventHotKey(keyRef) }
        if let handlerRef = handlerRef { RemoveEventHandler(handlerRef) }
    }
}
