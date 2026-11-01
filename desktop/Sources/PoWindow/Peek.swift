import AppKit

/// Where the window slides to when the pet peeks out from a screen edge.
enum Peek {
    /// The creature must be this close to an edge (in points) to peek from it.
    static let reach: CGFloat = 70
    /// How much of the creature goes off the screen.
    static let hidden: CGFloat = 0.55

    /// How far to move the window so the creature hangs off the nearest edge, or
    /// nil if it is not near one. Left and right are preferred to the bottom,
    /// where the Dock is.
    ///
    /// - creature: the creature's rectangle, in screen coordinates.
    /// - screen: the whole screen, including the menu bar and Dock.
    static func offset(creature: NSRect, screen: NSRect) -> NSPoint? {
        let gapRight = screen.maxX - creature.maxX
        let gapLeft = creature.minX - screen.minX
        let gapBottom = creature.minY - screen.minY

        let sides: [(gap: CGFloat, dx: CGFloat)] = [
            (gapRight, gapRight + creature.width * hidden),
            (gapLeft, -(gapLeft + creature.width * hidden)),
        ].filter { $0.gap <= reach && $0.gap >= -creature.width }
        if let nearest = sides.min(by: { $0.gap < $1.gap }) {
            return NSPoint(x: nearest.dx, y: 0)
        }
        if gapBottom <= reach && gapBottom >= -creature.height {
            return NSPoint(x: 0, y: -(gapBottom + creature.height * hidden))
        }
        return nil
    }

    static func selfTest() -> (passed: Int, failures: [String]) {
        var passed = 0
        var failures: [String] = []
        let screen = NSRect(x: 0, y: 0, width: 1710, height: 1107)
        func check(_ name: String, _ got: NSPoint?, _ want: NSPoint?) {
            let ok: Bool
            switch (got, want) {
            case (nil, nil): ok = true
            case let (g?, w?): ok = abs(g.x - w.x) < 0.01 && abs(g.y - w.y) < 0.01
            default: ok = false
            }
            if ok { passed += 1 } else { failures.append("\(name): got \(String(describing: got)), want \(String(describing: want))") }
        }
        // A 100 by 120 creature.
        func creature(x: CGFloat, y: CGFloat) -> NSRect { NSRect(x: x, y: y, width: 100, height: 120) }

        check("12 from the right edge", offset(creature: creature(x: 1598, y: 400), screen: screen), NSPoint(x: 67, y: 0))
        check("12 from the left edge", offset(creature: creature(x: 12, y: 400), screen: screen), NSPoint(x: -67, y: 0))
        check("12 from the bottom edge", offset(creature: creature(x: 800, y: 12), screen: screen), NSPoint(x: 0, y: -78))
        check("in the middle of the screen", offset(creature: creature(x: 800, y: 500), screen: screen), nil)
        check("a corner prefers the side to the bottom", offset(creature: creature(x: 1598, y: 12), screen: screen), NSPoint(x: 67, y: 0))
        // The creature's right side is 1640, so it is 70 from the edge: exactly the reach.
        check("exactly at the reach", offset(creature: creature(x: 1540, y: 400), screen: screen), NSPoint(x: 70 + 55, y: 0))
        check("one point beyond the reach", offset(creature: creature(x: 1539, y: 400), screen: screen), nil)
        check("touching the edge", offset(creature: creature(x: 1610, y: 400), screen: screen), NSPoint(x: 55, y: 0))
        // On a narrow screen both sides are near; the nearer (left, 15 away vs 45) wins.
        check("the nearer of two sides wins", offset(creature: creature(x: 15, y: 400), screen: NSRect(x: 0, y: 0, width: 160, height: 1107)), NSPoint(x: -(15 + 55), y: 0))
        return (passed, failures)
    }
}
