import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../config/router.dart';
import '../../providers/auth_provider.dart';

class DriverProfileScreen extends ConsumerStatefulWidget {
  const DriverProfileScreen({super.key});

  @override
  ConsumerState<DriverProfileScreen> createState() =>
      _DriverProfileScreenState();
}

class _DriverProfileScreenState extends ConsumerState<DriverProfileScreen> {
  String? _driverId;
  bool _loggingOut = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final id = await ref.read(currentDriverIdProvider.future);
      if (mounted) setState(() => _driverId = id);
    });
  }

  Future<void> _logout() async {
    if (_loggingOut) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Logout'),
        content: const Text('Yakin ingin keluar dari akun driver?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Logout'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _loggingOut = true);
    await ref.read(authProvider.notifier).logout();
    if (mounted) context.go(AppRoutes.login);
  }

  @override
  Widget build(BuildContext context) {
    final profile = ref.watch(driverProfileProvider).value;
    final driverId = _driverId ?? profile?.driverId;
    final driverIdText = driverId;

    return Scaffold(
      appBar: AppBar(title: const Text('Profil Driver')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _HeaderCard(
            name: profile?.name,
            email: profile?.email,
            driverId: driverId,
          ),
          const SizedBox(height: 16),
          _SectionCard(
            title: 'Akun',
            caption: 'Status diverifikasi saat login (backend menolak akun non-aktif).',
            children: [
              _InfoRow(
                icon: Icons.email_outlined,
                label: 'Email',
                value: profile?.email ?? '—',
              ),
              const Divider(height: 1),
              _InfoRow(
                icon: Icons.phone_iphone_outlined,
                label: 'Phone',
                value: _maskPhone(profile?.phone),
              ),
              const Divider(height: 1),
              _InfoRow(
                icon: Icons.badge_outlined,
                label: 'ID Driver',
                value: driverIdText == null || driverIdText.isEmpty
                    ? '—'
                    : _shortId(driverIdText),
              ),
              const Divider(height: 1),
              _InfoRow(
                icon: Icons.verified_outlined,
                label: 'Status',
                value: profile?.status.isNotEmpty == true
                    ? profile!.status
                    : '—',
                trailing: StatusChip(status: _statusLabel(profile?.status)),
              ),
            ],
          ),
          const SizedBox(height: 16),
          _SectionCard(
            title: 'Kendaraan',
            caption: profile?.hasVehicleInfo == true
                ? 'Data profil driver.'
                : 'Data kendaraan belum tersedia.',
            children: [
              _InfoRow(
                icon: Icons.directions_bike_outlined,
                label: 'Tipe kendaraan',
                value: _orDash(profile?.vehicleType),
              ),
              const Divider(height: 1),
              _InfoRow(
                icon: Icons.pin_outlined,
                label: 'Plat nomor',
                value: _orDash(profile?.vehiclePlate),
              ),
            ],
          ),
          const SizedBox(height: 16),
          _SectionCard(
            title: 'Performa',
            caption: 'Data performa dari server.',
            children: [
              _InfoRow(
                icon: Icons.star_border,
                label: 'Rating',
                value: _formatRating(profile?.ratingAvg ?? 0),
              ),
              const Divider(height: 1),
              _InfoRow(
                icon: Icons.route_outlined,
                label: 'Total rides',
                value: _formatTotalRides(profile?.totalRides ?? 0),
              ),
            ],
          ),
          const SizedBox(height: 24),
          OutlinedButton.icon(
            style: OutlinedButton.styleFrom(
              minimumSize: const Size.fromHeight(48),
              foregroundColor: Colors.red.shade700,
              side: BorderSide(color: Colors.red.shade200),
            ),
            onPressed: _loggingOut ? null : _logout,
            icon: _loggingOut
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.logout),
            label: const Text('Logout'),
          ),
        ],
      ),
    );
  }

  static String _orDash(String? v) => (v == null || v.trim().isEmpty) ? '—' : v;
  static String _formatRating(double value) =>
      value <= 0 ? '—' : value.toStringAsFixed(1);
  static String _formatTotalRides(int value) => value <= 0 ? '—' : '$value';
  static String _statusLabel(String? status) {
    if (status == null || status.isEmpty) return 'DRIVER AKTIF';
    if (status.toUpperCase() == 'ACTIVE') return 'DRIVER AKTIF';
    return status;
  }

  static String _shortId(String id) =>
      id.length > 8 ? '${id.substring(0, 8).toUpperCase()}…' : id.toUpperCase();

  /// Mask nomor telepon: hanya 4 digit awal + 3 digit akhir terlihat (PII).
  static String _maskPhone(String? v) {
    final p = (v ?? '').trim();
    if (p.length <= 7) return _orDash(v);
    return '${p.substring(0, 4)}****${p.substring(p.length - 3)}';
  }
}

class StatusChip extends StatelessWidget {
  const StatusChip({super.key, this.status = 'DRIVER AKTIF'});

  final String status;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: Colors.green.shade50,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: Colors.green.shade300),
      ),
      child: Text(
        status,
        style: TextStyle(
          color: Colors.green.shade800,
          fontSize: 11,
          fontWeight: FontWeight.bold,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}

class _HeaderCard extends StatelessWidget {
  const _HeaderCard({this.name, this.email, this.driverId});

  final String? name;
  final String? email;
  final String? driverId;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          CircleAvatar(
            radius: 28,
            backgroundColor: theme.colorScheme.primaryContainer,
            child: Icon(
              Icons.person,
              size: 32,
              color: theme.colorScheme.onPrimaryContainer,
            ),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  (name == null || name!.trim().isEmpty)
                      ? 'Driver G-Flow'
                      : name!,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  email == null || email!.isEmpty ? '—' : email!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: Colors.grey,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
                if (driverId != null && driverId!.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    'ID ${driverId!}',
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.outline,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  const _SectionCard({
    required this.title,
    required this.children,
    this.caption,
  });

  final String title;
  final List<Widget> children;
  final String? caption;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.titleSmall?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          ...children,
          if (caption != null) ...[
            const SizedBox(height: 10),
            Text(
              caption!,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontStyle: FontStyle.italic,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({
    required this.icon,
    required this.label,
    required this.value,
    this.trailing,
  });

  final IconData icon;
  final String label;
  final String value;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Icon(icon, size: 20, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: 10),
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
            ),
          ),
          Expanded(
            child: Text(
              value,
              textAlign: TextAlign.end,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          if (trailing != null) ...[const SizedBox(width: 8), trailing!],
        ],
      ),
    );
  }
}
