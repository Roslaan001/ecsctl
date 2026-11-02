class Ecsctl < Formula
  desc "A CLI tool for managing Amazon ECS resources, inspired by eksctl"
  homepage "https://roslaan001.github.io/ecsctl/"
  url "https://github.com/Roslaan001/ecsctl.git",
      tag:      "v0.5.1",
      revision: "2d98dc19be66196896ebabdea046c3d27f3cd6b5"
  license "Apache-2.0"
  head "https://github.com/Roslaan001/ecsctl.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/roslaan001/ecsctl/cmd.Version=#{version}
      -X github.com/roslaan001/ecsctl/cmd.Commit=#{Utils.git_short_head}
      -X github.com/roslaan001/ecsctl/cmd.BuildDate=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags: ldflags), "."

    generate_completions_from_executable(bin/"ecsctl", "completion")
  end

  test do
    assert_match "ecsctl is a command-line tool", shell_output("#{bin}/ecsctl --help")
    assert_match version.to_s, shell_output("#{bin}/ecsctl version")
  end
end
