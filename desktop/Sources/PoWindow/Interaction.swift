import AppKit

/// How the pet behaves when the pointer is near it. The pet sits on top of
/// everything, so by default it must never be in the way: when the pointer comes
/// close it fades and lets clicks through to whatever is underneath. Holding
/// Option over the creature makes it solid, so it can be dragged or right-clicked.
enum Interaction {
    struct Outcome: Equatable {
        /// Whether the window should catch the mouse (otherwise clicks pass through).
        let grabbable: Bool
        let alpha: CGFloat
    }

    static let fadedAlpha: CGFloat = 0.3

    /// - near: the pointer is within the creature's box, with a little margin.
    /// - overCreature: the pointer is on a drawn pixel of the creature.
    /// - optionDown: the Option key is held.
    /// - alwaysGrab: the person chose to grab the pet with a plain click.
    /// - dragging: the pet is being dragged right now.
    static func decide(near: Bool, overCreature: Bool, optionDown: Bool, alwaysGrab: Bool, dragging: Bool) -> Outcome {
        if dragging { return Outcome(grabbable: true, alpha: 1) }
        if overCreature && (optionDown || alwaysGrab) { return Outcome(grabbable: true, alpha: 1) }
        // Not grabbable. Fade near the pointer, unless the person asked for a pet
        // that is always solid.
        return Outcome(grabbable: false, alpha: (near && !alwaysGrab) ? fadedAlpha : 1)
    }

    /// Checks the rules above; used by `po-window --selftest`.
    static func selfTest() -> (passed: Int, failures: [String]) {
        var passed = 0
        var failures: [String] = []
        func check(_ name: String, _ got: Outcome, _ want: Outcome) {
            if got == want { passed += 1 } else { failures.append("\(name): got \(got), want \(want)") }
        }
        let solid = Outcome(grabbable: true, alpha: 1)
        let ghost = Outcome(grabbable: false, alpha: fadedAlpha)
        let clear = Outcome(grabbable: false, alpha: 1)

        check("far away", decide(near: false, overCreature: false, optionDown: false, alwaysGrab: false, dragging: false), clear)
        check("far away, option held", decide(near: false, overCreature: false, optionDown: true, alwaysGrab: false, dragging: false), clear)
        check("near but not on the creature", decide(near: true, overCreature: false, optionDown: false, alwaysGrab: false, dragging: false), ghost)
        check("option held near, not on the creature", decide(near: true, overCreature: false, optionDown: true, alwaysGrab: false, dragging: false), ghost)
        check("on the creature, plain", decide(near: true, overCreature: true, optionDown: false, alwaysGrab: false, dragging: false), ghost)
        check("on the creature, option held", decide(near: true, overCreature: true, optionDown: true, alwaysGrab: false, dragging: false), solid)
        check("dragging keeps hold even if the pointer outruns it", decide(near: false, overCreature: false, optionDown: false, alwaysGrab: false, dragging: true), solid)
        check("always grab: on the creature", decide(near: true, overCreature: true, optionDown: false, alwaysGrab: true, dragging: false), solid)
        check("always grab: near but not on it does not fade", decide(near: true, overCreature: false, optionDown: false, alwaysGrab: true, dragging: false), clear)
        check("always grab: far away", decide(near: false, overCreature: false, optionDown: false, alwaysGrab: true, dragging: false), clear)
        return (passed, failures)
    }
}
