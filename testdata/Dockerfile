# Systemd-enabled Docker image for integration testing
# See docker-systemd.md in the project root for requirements on running this container
# (requires --privileged, --cgroupns=host, tmpfs mounts, and cgroup bind mount)

FROM ubuntu:24.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install systemd and required utilities
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
        curl \
        ca-certificates \
        iproute2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd for container environment (remove unnecessary services)
RUN cd /lib/systemd/system/sysinit.target.wants/ && \
    ls | grep -v systemd-tmpfiles-setup | xargs rm -f && \
    rm -f /lib/systemd/system/multi-user.target.wants/* && \
    rm -f /etc/systemd/system/*.wants/* && \
    rm -f /lib/systemd/system/local-fs.target.wants/* && \
    rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
    rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
    rm -f /lib/systemd/system/basic.target.wants/* && \
    rm -f /lib/systemd/system/anaconda.target.wants/* 2>/dev/null || true

# Set systemd as the entrypoint
CMD ["/lib/systemd/systemd"]
