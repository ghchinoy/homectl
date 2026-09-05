// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import catppuccin from '@catppuccin/starlight';

export default defineConfig({
  site: 'https://ghchinoy.github.io',
  base: '/homectl',
  integrations: [
    starlight({
      title: 'homectl',
      description: 'Modern Go-powered local smart home management toolkit',
      logo: {
        src: './src/assets/homectl.svg',
      },
      plugins: [
        catppuccin({
          dark: { flavor: 'mocha', accent: 'sky' },
          light: { flavor: 'latte', accent: 'sky' },
        }),
      ],
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/ghchinoy/homectl',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/ghchinoy/homectl/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction & Quickstart', slug: 'guides/getting-started' },
            { label: 'Architecture Overview', slug: 'guides/architecture' },
            { label: 'Installation', slug: 'guides/installation' },
          ],
        },
        {
          label: 'Architecture Deep Dives',
          items: [
            { label: 'System Architecture (Overall)', slug: 'architecture' },
            { label: 'Sonos Audio Vertical', slug: 'architecture/sonos' },
            { label: 'Lutron Lighting Vertical', slug: 'architecture/lutron' },
            { label: 'Qolsys Security Vertical', slug: 'architecture/qolsys' },
            { label: 'Google Cast Vertical', slug: 'architecture/google-cast' },
            { label: 'Security Camera Vertical', slug: 'architecture/cameras' },
            { label: 'Maintaining Diagrams & Docs', slug: 'architecture/maintenance' },
          ],
        },
        {
          label: 'Hardware Integrations',
          items: [
            { label: 'Lutron Caseta & RA2 Select', slug: 'integrations/lutron' },
            { label: 'Sonos Whole-Home Audio', slug: 'integrations/sonos' },
            { label: 'Google Cast', slug: 'integrations/google-cast' },
            { label: 'Security Cameras (RTSP/ONVIF)', slug: 'integrations/cameras' },
            { label: 'Qolsys IQ Panel', slug: 'integrations/qolsys' },
          ],
        },
        {
          label: 'User Interfaces',
          items: [
            { label: 'Terminal UI (Bubble Tea)', slug: 'interfaces/tui' },
            { label: 'CLI Reference', slug: 'interfaces/cli' },
            { label: 'Web UI & REST API', slug: 'interfaces/web-api' },
          ],
        },
        {
          label: 'Operations & Reference',
          items: [
            { label: 'Configuration & Nicknames', slug: 'reference/configuration' },
            { label: 'Systemd Service Deployment', slug: 'reference/service-deployment' },
            { label: 'Network Topology & NAT', slug: 'reference/network-topology' },
            { label: 'Developer Guide & Beads', slug: 'reference/developer-guide' },
          ],
        },
      ],
    }),
  ],
});
