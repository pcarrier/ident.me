const std = @import("std");

const targets: []const std.Target.Query = &.{
    .{ .cpu_arch = .aarch64, .os_tag = .macos },
    .{ .cpu_arch = .aarch64, .os_tag = .windows },
    .{ .cpu_arch = .x86_64, .os_tag = .windows },
    .{ .cpu_arch = .aarch64, .os_tag = .linux, .abi = .gnu },
    .{ .cpu_arch = .aarch64, .os_tag = .linux, .abi = .musl },
    .{ .cpu_arch = .x86_64, .os_tag = .linux, .abi = .gnu },
    .{ .cpu_arch = .x86_64, .os_tag = .linux, .abi = .musl },
};

pub fn build(b: *std.Build) !void {
    for (targets) |t| {
        const exe = b.addExecutable(.{
            .name = "identme",
            .target = b.resolveTargetQuery(t),
        });
        exe.addCSourceFile(.{
            .file = b.path("main.cpp"),
            .flags = &.{ "-std=c++17", "-Os" },
        });
        if (t.os_tag == .windows) {
            exe.linkSystemLibrary("winhttp");
        } else {
            exe.linkSystemLibrary("curl");
        }
        const output = b.addInstallArtifact(exe, .{ .dest_dir = .{ .override = .{
            .custom = try t.zigTriple(b.allocator),
        } } });
        b.getInstallStep().dependOn(&output.step);
    }
}
