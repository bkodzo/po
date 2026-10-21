// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "PoWindow",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(name: "PoWindow", path: "Sources/PoWindow"),
    ]
)
