// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "PoWindow",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "po-window", targets: ["PoWindow"]),
    ],
    targets: [
        .executableTarget(name: "PoWindow", path: "Sources/PoWindow"),
    ]
)
