# NixOS Module for VerificationBot
# ─────────────────────────────────
# Usage: import this file in your configuration.nix, or
# add it to imports = [ ./verificationbot.nix ];
#
# Then set:
#   services.verificationbot.enable = true;
#   services.verificationbot.envFile = "/run/secrets/verificationbot.env";

{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.verificationbot;

  # Build the Go binary from source.
  # Alternatively, use a pre-built binary and skip this derivation.
  verificationbot = pkgs.buildGoModule {
    pname = "verificationbot";
    version = "1.0.0";

    src = pkgs.fetchFromGitHub {
      owner  = "your-github-username";
      repo   = "VerificationBot";
      rev    = "main"; # or a specific commit hash
      # Run `nix-prefetch-github your-username VerificationBot` to get this:
      sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    };

    # Run `go mod vendor` locally, then:
    # vendorHash = pkgs.lib.fakeHash;  # first build, then replace with real hash
    vendorHash = "sha256-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=";

    CGO_ENABLED = "0";
    ldflags = [ "-s" "-w" "-extldflags=-static" ];
  };

in {
  options.services.verificationbot = {
    enable   = mkEnableOption "VerificationBot Telegram verification bot";
    package  = mkOption { type = types.package; default = verificationbot; };
    envFile  = mkOption {
      type        = types.str;
      description = "Path to the .env file containing secrets (TELEGRAM_TOKEN, etc.)";
      example     = "/run/secrets/verificationbot.env";
    };
    dataDir = mkOption {
      type    = types.str;
      default = "/var/lib/verificationbot";
    };
  };

  config = mkIf cfg.enable {
    users.users.verificationbot = {
      isSystemUser = true;
      group        = "verificationbot";
      description  = "VerificationBot service user";
      home         = cfg.dataDir;
      createHome   = true;
    };
    users.groups.verificationbot = {};

    systemd.services.verificationbot = {
      description = "VerificationBot Telegram Bot";
      after       = [ "network-online.target" ];
      wants       = [ "network-online.target" ];
      wantedBy    = [ "multi-user.target" ];

      serviceConfig = {
        Type             = "simple";
        User             = "verificationbot";
        Group            = "verificationbot";
        WorkingDirectory = cfg.dataDir;
        EnvironmentFile  = cfg.envFile;
        ExecStart        = "${cfg.package}/bin/bot";
        Restart          = "on-failure";
        RestartSec       = "5s";

        # Hardening
        NoNewPrivileges  = true;
        PrivateTmp       = true;
        ProtectSystem    = "strict";
        ReadWritePaths   = [ cfg.dataDir ];
      };
    };
  };
}
