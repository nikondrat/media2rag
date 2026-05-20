// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "Tuist",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(url: "https://github.com/tuist/tuist", from: "4.141.0"),
    ],
    targets: [
        .target(
            name: "Project",
            dependencies: [
                .product(name: "ProjectDescription", package: "tuist")
            ]
        )
    ]
)
