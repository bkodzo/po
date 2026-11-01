import AppKit
import Carbon.HIToolbox
import CoreGraphics

// Po's desktop window: a small transparent pet that floats above other windows,
// never takes focus, lets clicks through where it is transparent, can be dragged
// and snaps to screen edges. It owns no logic: it plays the theme Po exported
// and follows the state Po streams.
//
//   po-window                  show the pet (Po must be running)
//   po-window --level popup    try another window level
//   po-window --scale 4        pixels per sprite pixel
//   po-window --seconds 10     quit by itself after a while
//   po-window --listen         no window: print what Po streams, to check the link
//   po-window --log            print the current mood and frame once a second
//   po-window --probe          print display info and what is readable about other windows
//   po-window --selftest       check the pointer and grab rules
//
// Hold Option over the creature to grab it; Control+Option+P hides or shows it.

struct Options {
    var level = "status"
    var scale = 5
    var seconds: Double? = nil
    var probe = false
    var listen = false
    var log = false
    var selftest = false
    var scaleGiven = false
}

func parseOptions() -> Options {
    var o = Options()
    var args = Array(CommandLine.arguments.dropFirst())
    while !args.isEmpty {
        let a = args.removeFirst()
        switch a {
        case "--level": if !args.isEmpty { o.level = args.removeFirst() }
        case "--scale": if !args.isEmpty { o.scale = max(1, Int(args.removeFirst()) ?? 5); o.scaleGiven = true }
        case "--seconds": if !args.isEmpty { o.seconds = Double(args.removeFirst()) }
        case "--probe": o.probe = true
        case "--listen": o.listen = true
        case "--log": o.log = true
        case "--selftest": o.selftest = true
        default: break
        }
    }
    return o
}

func windowLevel(named name: String) -> NSWindow.Level {
    switch name {
    case "normal": return .normal
    case "floating": return .floating
    case "status": return .statusBar
    case "popup": return .popUpMenu
    case "screensaver": return .screenSaver
    case "shield": return NSWindow.Level(rawValue: Int(CGShieldingWindowLevel()))
    default: return .statusBar
    }
}

// MARK: - The view

final class PetView: NSView {
    var cache: FrameCache?
    var mood = "calm"
    var tick = 0
    /// The idle animation playing now, if any, and how far into it.
    var activity: String?
    var activityTick = 0
    var line: String?
    var lineShownAt = Date.distantPast
    var dragging = false
    var onDragEnd: (() -> Void)?
    var onDragStart: (() -> Void)?

    var scale: Int
    var menuProvider: (() -> NSMenu)?
    private var grabOffset = NSPoint.zero

    static let bubbleRoom: CGFloat = 64
    static let minWidth: CGFloat = 260
    static let lineLife: TimeInterval = 4

    init(scale: Int) {
        self.scale = scale
        super.init(frame: NSRect(origin: .zero, size: PetView.size(scale: scale, sprite: (20, 24))))
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    static func size(scale: Int, sprite: (Int, Int)) -> NSSize {
        NSSize(width: max(minWidth, CGFloat(sprite.0 * scale)), height: CGFloat(sprite.1 * scale) + bubbleRoom)
    }

    var spriteSize: (Int, Int) {
        guard let anim = cache?.animation(for: "calm") else { return (20, 24) }
        return (anim.w, anim.h)
    }

    /// Where the creature is drawn: the bottom-right of the view, so the speech
    /// bubble has room above and to its left.
    var petRect: NSRect {
        let (w, h) = spriteSize
        return NSRect(x: bounds.width - CGFloat(w * scale), y: 0, width: CGFloat(w * scale), height: CGFloat(h * scale))
    }

    override func draw(_ dirtyRect: NSRect) {
        let idleImage = activity.flatMap { cache?.activityImage(name: $0, tick: activityTick) }
        guard let ctx = NSGraphicsContext.current?.cgContext,
              let image = idleImage ?? cache?.image(mood: mood, tick: tick) else { return }
        ctx.interpolationQuality = .none
        ctx.draw(image, in: petRect)
        drawBubble()
    }

    private func drawBubble() {
        guard let line = line, !line.isEmpty else { return }
        let age = Date().timeIntervalSince(lineShownAt)
        guard age < PetView.lineLife else { return }
        let alpha = CGFloat(age > PetView.lineLife - 0.6 ? (PetView.lineLife - age) / 0.6 : 1)

        let text = NSAttributedString(string: line, attributes: [
            .font: NSFont.systemFont(ofSize: 12, weight: .medium),
            .foregroundColor: NSColor(white: 0.1, alpha: alpha),
        ])
        let measured = text.boundingRect(with: NSSize(width: 170, height: 200), options: [.usesLineFragmentOrigin])
        let pad: CGFloat = 8
        let box = NSRect(
            x: max(4, bounds.width - ceil(measured.width) - 2 * pad - 6),
            y: petRect.maxY + 4,
            width: ceil(measured.width) + 2 * pad,
            height: ceil(measured.height) + 2 * pad)
        let path = NSBezierPath(roundedRect: box, xRadius: 8, yRadius: 8)
        NSColor(white: 1, alpha: 0.95 * alpha).setFill()
        path.fill()
        NSColor(white: 0.3, alpha: alpha).setStroke()
        path.lineWidth = 1
        path.stroke()
        text.draw(with: box.insetBy(dx: pad, dy: pad), options: [.usesLineFragmentOrigin])
    }

    /// Whether a point in this view lands on a drawn pixel of the creature.
    func isSolid(at point: NSPoint) -> Bool {
        let rect = petRect
        guard rect.contains(point) else { return false }
        let x = Int((point.x - rect.minX) / CGFloat(scale))
        let fromBottom = Int((point.y - rect.minY) / CGFloat(scale))
        let (_, h) = spriteSize
        if let name = activity, cache?.activityLength(name) != nil {
            return cache?.activityIsSolid(name: name, tick: activityTick, x: x, y: h - 1 - fromBottom) ?? false
        }
        return cache?.isSolid(mood: mood, tick: tick, x: x, y: h - 1 - fromBottom) ?? false
    }

    // Dragging: the window follows the pointer, and snaps to an edge on release.

    override func mouseDown(with event: NSEvent) {
        onDragStart?()
        dragging = true
        window?.ignoresMouseEvents = false
        grabOffset = event.locationInWindow
    }

    override func mouseDragged(with event: NSEvent) {
        guard let window = window else { return }
        let p = NSEvent.mouseLocation
        window.setFrameOrigin(NSPoint(x: p.x - grabOffset.x, y: p.y - grabOffset.y))
    }

    override func mouseUp(with event: NSEvent) {
        dragging = false
        snapToEdges()
        onDragEnd?()
    }

    override func rightMouseDown(with event: NSEvent) {
        guard let menu = menuProvider?() else { return }
        NSMenu.popUpContextMenu(menu, with: event, for: self)
    }

    /// The creature, not the whole transparent window, is what snaps to an edge.
    func snapToEdges() {
        guard let window = window, let screen = window.screen ?? NSScreen.main else { return }
        let area = screen.visibleFrame
        let margin: CGFloat = 12
        let reach: CGFloat = 32
        let pet = petRect
        var origin = window.frame.origin
        let petLeft = origin.x + pet.minX, petRight = origin.x + pet.maxX
        let petBottom = origin.y + pet.minY, petTop = origin.y + pet.maxY
        if abs(petLeft - area.minX) < reach { origin.x += area.minX + margin - petLeft }
        if abs(area.maxX - petRight) < reach { origin.x += area.maxX - margin - petRight }
        if abs(petBottom - area.minY) < reach { origin.y += area.minY + margin - petBottom }
        if abs(area.maxY - petTop) < reach { origin.y += area.maxY - margin - petTop }
        window.setFrameOrigin(origin)
    }
}

// MARK: - The app

final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    let options: Options
    var panel: NSPanel!
    var view: PetView!
    let stream = StateStream()
    var seenLineSeq: Int?
    var lastStatus = ""
    var lastFrameKey = ""
    /// The sequence number of the idle animation last started, so one that has
    /// already played is not played again while Po is still announcing it.
    var playedActivitySeq = -1
    var connected = false
    var userHidden = false
    var alwaysGrab = UserDefaults.standard.bool(forKey: "alwaysGrab")
    var targetAlpha: CGFloat = 1
    /// Where the window was before a peek, so it can go back.
    var peekHome: NSPoint?
    var statusItem: NSStatusItem?
    var hotKey: HotKey?
    var lastIcon = ""
    var interaction = "far"
    // Counters for --log, to see where the time goes.
    var ticksSinceLog = 0, redrawsSinceLog = 0, mouseSinceLog = 0

    init(options: Options) { self.options = options }

    func applicationDidFinishLaunching(_ notification: Notification) {
        let saved = UserDefaults.standard.integer(forKey: "scale")
        view = PetView(scale: options.scaleGiven ? options.scale : (saved > 0 ? saved : options.scale))
        view.onDragEnd = { [weak self] in self?.savePosition() }
        view.onDragStart = { [weak self] in self?.endPeek(animated: false) }
        view.menuProvider = { [weak self] in self?.makeMenu() ?? NSMenu() }

        panel = NSPanel(
            contentRect: NSRect(origin: .zero, size: view.frame.size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered, defer: false)
        panel.contentView = view
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = false
        panel.isFloatingPanel = true // this resets the level, so set the level after it
        panel.level = windowLevel(named: options.level)
        panel.hidesOnDeactivate = false
        panel.worksWhenModal = true
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary, .ignoresCycle]
        panel.acceptsMouseMovedEvents = true
        panel.ignoresMouseEvents = true // until the pointer is over the creature
        restorePosition()

        // Clicks pass through, and the pet fades when the pointer is near. Mouse
        // moves reach us even while the window ignores the mouse, through the
        // global monitor; holding Option is noticed on the animation tick.
        NSEvent.addGlobalMonitorForEvents(matching: [.mouseMoved, .leftMouseDragged]) { [weak self] _ in self?.updateInteraction() }
        NSEvent.addLocalMonitorForEvents(matching: [.mouseMoved]) { [weak self] event in
            self?.updateInteraction()
            return event
        }

        setUpStatusItem()
        hotKey = HotKey(keyCode: kVK_ANSI_P, modifiers: controlKey | optionKey) { [weak self] in self?.toggleHidden() }
        print(hotKey == nil ? "hotkey Control+Option+P could not be registered (another app may use it)"
                            : "hotkey Control+Option+P hides or shows the pet; hold Option over it to grab it")

        stream.onStatus = { [weak self] status in self?.handle(status: status) }
        stream.onState = { [weak self] state in self?.apply(state) }
        stream.start() // the window appears once it is connected to Po

        // One tick per frame of the animation. The theme says how fast that is.
        let fps = max(1, view.cache?.pack.fps["full"] ?? 8)
        Timer.scheduledTimer(withTimeInterval: 1.0 / Double(fps), repeats: true) { [weak self] _ in self?.advance() }
        if options.log {
            Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in self?.logFrame() }
        }

        if let seconds = options.seconds {
            DispatchQueue.main.asyncAfter(deadline: .now() + seconds) {
                print("done after \(seconds)s")
                NSApp.terminate(nil)
            }
        }
        print("Po window started: level \(options.level) (\(panel.level.rawValue)), scale \(view.scale)")
    }

    /// Reads the theme the pet exported. Returns whether there is one to play.
    @discardableResult
    func loadTheme() -> Bool {
        do {
            let pack = try ThemePack.load(from: PoPaths.theme)
            view.cache = FrameCache(pack: pack)
            resize(toScale: view.scale)
            updateStatusIcon()
            return true
        } catch {
            if lastStatus != "no theme" {
                lastStatus = "no theme"
                print("no theme yet (\(error.localizedDescription)); Po writes it when it starts")
            }
            return false
        }
    }

    func handle(status: String) {
        if status != lastStatus { print("po: \(status)") }
        lastStatus = status
        switch status {
        case "connected":
            seenLineSeq = nil // the first state after connecting is not news
            playedActivitySeq = -1
            connected = loadTheme()
        default:
            // Po is not there, so there is nothing to show. The window comes back
            // by itself when Po does.
            connected = false
        }
        refreshVisibility()
    }

    /// The pet shows while Po is there and the person has not hidden it.
    func refreshVisibility() {
        if !connected || userHidden { endPeek(animated: false) }
        if connected && !userHidden {
            panel.orderFrontRegardless()
        } else {
            panel.orderOut(nil)
        }
    }

    @objc func toggleHidden() {
        userHidden.toggle()
        print(userHidden ? "hidden (Control+Option+P shows it again)" : "shown")
        refreshVisibility()
    }

    /// Changes the size, keeping the creature's bottom-right corner where it is.
    func resize(toScale n: Int) {
        view.scale = n
        let size = PetView.size(scale: n, sprite: view.spriteSize)
        let bottom = panel.frame.minY
        let right = panel.frame.maxX
        view.setFrameSize(size)
        panel.setContentSize(size)
        panel.setFrameOrigin(NSPoint(x: right - size.width, y: bottom))
        view.needsDisplay = true
    }

    @objc func chooseSize(_ item: NSMenuItem) {
        resize(toScale: item.tag)
        UserDefaults.standard.set(item.tag, forKey: "scale")
        savePosition()
    }

    @objc func toggleAlwaysGrab() {
        alwaysGrab.toggle()
        UserDefaults.standard.set(alwaysGrab, forKey: "alwaysGrab")
        updateInteraction()
    }

    // MARK: Menus

    func makeMenu() -> NSMenu {
        let menu = NSMenu()
        fill(menu)
        return menu
    }

    func fill(_ menu: NSMenu) {
        let toggle = NSMenuItem(title: (userHidden ? "Show Po" : "Hide Po") + "    \u{2303}\u{2325}P", action: #selector(toggleHidden), keyEquivalent: "")
        toggle.target = self
        menu.addItem(toggle)

        let sizeItem = NSMenuItem(title: "Size", action: nil, keyEquivalent: "")
        let sizes = NSMenu()
        for (name, value) in [("Small", 3), ("Medium", 4), ("Large", 5), ("Extra large", 6)] {
            let it = NSMenuItem(title: name, action: #selector(chooseSize(_:)), keyEquivalent: "")
            it.target = self
            it.tag = value
            it.state = view.scale == value ? .on : .off
            sizes.addItem(it)
        }
        sizeItem.submenu = sizes
        menu.addItem(sizeItem)

        let grab = NSMenuItem(title: "Grab with a plain click (no Option key)", action: #selector(toggleAlwaysGrab), keyEquivalent: "")
        grab.target = self
        grab.state = alwaysGrab ? .on : .off
        menu.addItem(grab)

        menu.addItem(.separator())
        menu.addItem(NSMenuItem(title: "Quit Po window", action: #selector(NSApplication.terminate(_:)), keyEquivalent: ""))
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()
        fill(menu)
    }

    func setUpStatusItem() {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        let menu = NSMenu()
        menu.delegate = self
        item.menu = menu
        statusItem = item
        updateStatusIcon()
    }

    /// The menu bar icon is the creature's current face, so it follows the mood.
    func updateStatusIcon() {
        guard let item = statusItem, let cache = view.cache, let anim = cache.animation(for: view.mood) else { return }
        let key = "\(view.mood)/\(anim.sequence.first ?? 0)"
        guard key != lastIcon, let image = cache.image(mood: view.mood, tick: 0) else { return }
        lastIcon = key
        // Drawn at half size so each pixel is one device pixel on a Retina screen.
        let icon = NSImage(cgImage: image, size: NSSize(width: Double(image.width) / 2, height: Double(image.height) / 2))
        icon.isTemplate = false
        item.button?.image = icon
        item.button?.toolTip = "Po (Control+Option+P hides or shows it)"
    }

    func apply(_ state: PetState) {
        if state.mood != view.mood {
            view.mood = state.mood
            view.tick = 0
            updateStatusIcon()
        }
        applyActivity(state)
        if let seen = seenLineSeq {
            if state.lineSeq > seen, let line = state.line, !line.isEmpty {
                view.line = line
                view.lineShownAt = Date()
            }
        }
        seenLineSeq = state.lineSeq
        view.needsDisplay = true
    }

    /// Starts or stops an idle animation as Po says. A new sequence number means
    /// a new animation; the same number with nothing playing means it has already
    /// finished here.
    func applyActivity(_ state: PetState) {
        guard let name = state.activity, !name.isEmpty else {
            if view.activity != nil { view.activity = nil; view.needsDisplay = true }
            endPeek()
            return
        }
        let seq = state.activitySeq ?? 0
        guard seq != playedActivitySeq, view.cache?.activityLength(name) != nil else { return }
        playedActivitySeq = seq
        endPeek(animated: false) // a different animation replaces a peek
        view.activity = name
        view.activityTick = 0
        view.needsDisplay = true
        if name == "peek" { startPeek() }
    }

    // MARK: Peeking

    /// Slides the window so the creature hangs partly off the nearest screen
    /// edge, if it is next to one. Otherwise the pet just glances around.
    func startPeek() {
        guard peekHome == nil, panel.isVisible, !view.dragging,
              let screen = panel.screen ?? NSScreen.main else { return }
        let creature = view.petRect.offsetBy(dx: panel.frame.minX, dy: panel.frame.minY)
        guard let offset = Peek.offset(creature: creature, screen: screen.frame) else { return }
        let home = panel.frame.origin
        peekHome = home
        move(to: NSPoint(x: home.x + offset.x, y: home.y + offset.y), over: 0.7)
    }

    func endPeek(animated: Bool = true) {
        guard let home = peekHome else { return }
        peekHome = nil
        move(to: home, over: animated ? 0.6 : 0)
    }

    func move(to origin: NSPoint, over seconds: TimeInterval) {
        let frame = NSRect(origin: origin, size: panel.frame.size)
        guard seconds > 0 else {
            panel.setFrame(frame, display: true)
            return
        }
        NSAnimationContext.runAnimationGroup { ctx in
            ctx.duration = seconds
            ctx.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
            panel.animator().setFrame(frame, display: true)
        }
    }

    /// Moves the animation on one tick, and only redraws if what is on screen
    /// would change: a new picture, or a speech bubble that is showing or fading.
    func advance() {
        guard let cache = view.cache, let anim = cache.animation(for: view.mood), !anim.sequence.isEmpty else { return }
        view.tick += 1
        ticksSinceLog += 1
        if panel.isVisible && (panel.frame.contains(NSEvent.mouseLocation) || targetAlpha != 1) { updateInteraction() }

        var key: String
        if let name = view.activity, let length = cache.activityLength(name) {
            view.activityTick += 1
            if name == "peek" && view.activityTick == length.ticks - 6 { endPeek() }
            if !length.loop && view.activityTick >= length.ticks {
                view.activity = nil // a one-off has finished; back to the mood's loop
                key = "\(view.mood)/\(anim.sequence[view.tick % anim.sequence.count])"
            } else {
                key = "act:\(name)/\(cache.activityFrameIndex(name: name, tick: view.activityTick) ?? 0)"
            }
        } else {
            key = "\(view.mood)/\(anim.sequence[view.tick % anim.sequence.count])"
        }
        let bubbleActive = view.line != nil && Date().timeIntervalSince(view.lineShownAt) < PetView.lineLife + 0.3
        if key != lastFrameKey || bubbleActive {
            lastFrameKey = key
            view.needsDisplay = true
            redrawsSinceLog += 1
        }
    }

    func logFrame() {
        guard let cache = view.cache, let anim = cache.animation(for: view.mood), !anim.sequence.isEmpty else {
            print("log: no theme loaded")
            return
        }
        let index = anim.sequence[view.tick % anim.sequence.count]
        let doing = view.activity.map { ", doing \($0) (tick \(view.activityTick))" } ?? ""
        print("log: mood \(view.mood)\(doing), tick \(view.tick), frame \(index) of \(anim.frames.count)\(view.line.map { ", line \"\($0)\"" } ?? "") | last second: \(ticksSinceLog) ticks, \(redrawsSinceLog) redraws, \(mouseSinceLog) mouse events | pointer: \(interaction), alpha \(String(format: "%.2f", Double(targetAlpha)))")
        ticksSinceLog = 0; redrawsSinceLog = 0; mouseSinceLog = 0
    }

    /// Decides, from where the pointer is, whether the pet catches the mouse and
    /// how solid it looks. See Interaction.
    func updateInteraction() {
        mouseSinceLog += 1
        let p = NSEvent.mouseLocation
        var near = false, over = false
        if panel.isVisible && panel.frame.contains(p) {
            let local = view.convert(panel.convertPoint(fromScreen: p), from: nil)
            near = view.petRect.insetBy(dx: -12, dy: -12).contains(local)
            over = view.isSolid(at: local)
        }
        let outcome = Interaction.decide(
            near: near, overCreature: over,
            optionDown: NSEvent.modifierFlags.contains(.option),
            alwaysGrab: alwaysGrab, dragging: view.dragging)
        interaction = outcome.grabbable ? "grabbable" : (near ? "faded, clicks pass through" : "far")
        if panel.ignoresMouseEvents == outcome.grabbable { panel.ignoresMouseEvents = !outcome.grabbable }
        if outcome.alpha != targetAlpha {
            targetAlpha = outcome.alpha
            NSAnimationContext.runAnimationGroup { ctx in
                ctx.duration = 0.15
                panel.animator().alphaValue = outcome.alpha
            }
        }
    }

    // The window remembers where it was left.

    func savePosition() {
        let o = panel.frame.origin
        UserDefaults.standard.set("\(o.x),\(o.y)", forKey: "origin")
    }

    func restorePosition() {
        if let saved = UserDefaults.standard.string(forKey: "origin") {
            let parts = saved.split(separator: ",").compactMap { Double($0) }
            if parts.count == 2 {
                let origin = NSPoint(x: parts[0], y: parts[1])
                let probe = NSRect(origin: origin, size: panel.frame.size)
                if NSScreen.screens.contains(where: { $0.visibleFrame.intersects(probe) }) {
                    panel.setFrameOrigin(origin)
                    return
                }
            }
        }
        if let area = NSScreen.main?.visibleFrame {
            panel.setFrameOrigin(NSPoint(x: area.maxX - panel.frame.width - 12, y: area.minY + 12))
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }
}

// MARK: - Listen and probe (no window)

/// Prints what Po streams, to check the link without drawing anything.
func listen(seconds: Double) {
    if let pack = try? ThemePack.load(from: PoPaths.theme) {
        let calm = pack.moods["calm"]?.full
        print("theme: \(pack.name), version \(pack.version), moods \(pack.moods.keys.sorted()), calm loop \(calm?.sequence.count ?? 0) ticks of \(calm?.frames.count ?? 0) frames")
    } else {
        print("theme: none found at \(PoPaths.theme.path)")
    }
    let stream = StateStream()
    stream.onStatus = { print("status: \($0)") }
    stream.onState = { s in print("state: mood \(s.mood), resting \(s.resting), line_seq \(s.lineSeq)\(s.line.map { ", line \"\($0)\"" } ?? "")") }
    stream.start()
    DispatchQueue.main.asyncAfter(deadline: .now() + seconds) { exit(0) }
    dispatchMain()
}

/// Reports what can be learned about other windows without any permission.
func probe() {
    _ = NSApplication.shared
    print("macOS \(ProcessInfo.processInfo.operatingSystemVersionString)")
    for (i, s) in NSScreen.screens.enumerated() {
        print("display \(i): frame \(s.frame), usable \(s.visibleFrame), scale \(s.backingScaleFactor)x")
    }
    print("screen recording permission for this program: \(CGPreflightScreenCaptureAccess())")

    let raw = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    let normal = raw.filter { ($0[kCGWindowLayer as String] as? Int) == 0 }
    print("on-screen windows: \(raw.count) total, \(normal.count) ordinary")
    // Our own window may sit on any layer, so look for it across all of them.
    for w in raw where ((w[kCGWindowOwnerName as String] as? String) ?? "") == "po-window" {
        let b = (w[kCGWindowBounds as String] as? [String: Any]) ?? [:]
        print("  po-window window: layer \(w[kCGWindowLayer as String] ?? "?"), alpha \(w[kCGWindowAlpha as String] ?? "?"), x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")")
    }
    for w in normal.prefix(8) {
        let owner = (w[kCGWindowOwnerName as String] as? String) ?? "?"
        let bounds = (w[kCGWindowBounds as String] as? [String: Any]).map { b in
            "x \(b["X"] ?? "?") y \(b["Y"] ?? "?") w \(b["Width"] ?? "?") h \(b["Height"] ?? "?")"
        } ?? "no bounds"
        print("  \(owner): \(bounds)")
    }
}

// MARK: - Main

let options = parseOptions()

// Show log lines as they happen even when output goes to a file, as it does
// when the login agent runs the window.
setvbuf(stdout, nil, _IOLBF, 0)

/// Signals that should end the window cleanly, with a normal exit status.
///
/// Ctrl+Z would suspend the program and leave its window frozen on screen, with
/// nothing left running to remove it. And a plain "please stop" (SIGTERM) must
/// count as a normal exit, or the login agent would take it for a crash and
/// start the window again.
var signalWatchers: [DispatchSourceSignal] = []
func quitOnSignals() {
    for (sig, reason) in [(SIGTSTP, "Ctrl+Z received: quitting instead of freezing the window"),
                          (SIGTERM, "asked to stop: quitting")] {
        signal(sig, SIG_IGN)
        let watcher = DispatchSource.makeSignalSource(signal: sig, queue: .main)
        watcher.setEventHandler {
            print(reason)
            NSApp.terminate(nil)
        }
        watcher.resume()
        signalWatchers.append(watcher)
    }
}

if options.selftest {
    let (p1, f1) = Interaction.selfTest()
    let (p2, f2) = Peek.selfTest()
    let (passed, failures) = (p1 + p2, f1 + f2)
    failures.forEach { print("FAIL \($0)") }
    print("selftest: \(passed) passed, \(failures.count) failed")
    exit(failures.isEmpty ? 0 : 1)
} else if options.probe {
    probe()
} else if options.listen {
    listen(seconds: options.seconds ?? 10)
} else {
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    quitOnSignals()
    let delegate = AppDelegate(options: options)
    app.delegate = delegate
    app.run()
}
