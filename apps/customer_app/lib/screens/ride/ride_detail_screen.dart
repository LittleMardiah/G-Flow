import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/ride_order.dart';
import '../../providers/ride_provider.dart';

/// Ride Detail Screen (TD-072 C2, API_CONTRACT 7.4).
///
/// - Status badge (warna per status).
/// - Driver card — backend saat ini hanya mengekspos `driver_id` pada
///   GET /rides/{id} (internal/ride/handler.go:211), jadi ditampilkan short
///   ID driver; nama/telepon/kendaraan tercatat sebagai TD (usulan) karena
///   endpoint info driver belum tersedia.
/// - Route pickup → dropoff, fare breakdown, timestamps, metode pembayaran.
///
/// Argumen navigasi: `RideDetailArgs` (orderId).
class RideDetailScreen extends ConsumerWidget {
  const RideDetailScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final raw = ModalRoute.of(context)?.settings.arguments;
    if (raw is! RideDetailArgs) return const _MissingArgsView();

    final state = ref.watch(rideDetailProvider(raw.orderId));

    return Scaffold(
      appBar: AppBar(
        title: const Text('Detail Perjalanan'),
        actions: [
          Center(
            child: Padding(
              padding: const EdgeInsets.only(right: 12),
              child: Text(
                '#${_shortId(raw.orderId)}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ),
        ],
      ),
      body: _buildBody(context, state,
          onRetry: () => ref.read(rideDetailProvider(raw.orderId).notifier).load()),
    );
  }

  Widget _buildBody(
    BuildContext context,
    RideDetailState state, {
    required VoidCallback onRetry,
  }) {
    final order = state.order;
    if (order == null) {
      if (state.error != null) {
        return _ErrorView(message: state.error!, onRetry: onRetry);
      }
      return const Center(child: CircularProgressIndicator());
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _StatusCard(order: order),
          const SizedBox(height: 12),
          _DriverCard(order: order),
          const SizedBox(height: 12),
          _RouteCard(order: order),
          const SizedBox(height: 12),
          _FareCard(order: order),
          const SizedBox(height: 12),
          _TimelineCard(order: order),
          const SizedBox(height: 12),
          _PaymentCard(order: order),
        ],
      ),
    );
  }
}

// Status badge

class _StatusCard extends StatelessWidget {
  const _StatusCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = _statusColor(order.status);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(_statusIcon(order.status), size: 22, color: color),
            const SizedBox(width: 10),
            Expanded(
              child: Text(_statusText[order.status] ?? order.status, style: theme.textTheme.titleSmall),
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text(
                order.status.replaceAll('_', ' '),
                style: TextStyle(color: color, fontSize: 11, fontWeight: FontWeight.w600),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// Driver

class _DriverCard extends StatelessWidget {
  const _DriverCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final driver = order.driver;
    final driverId = order.driverId;
    final name = driver?.name.trim() ?? '';
    final displayName = name.isNotEmpty
        ? name
        : driverId != null
            ? 'Driver #${_shortId(driverId)}'
            : 'Belum ada driver';
    final vehicle = [
      driver?.vehicleType.trim() ?? '',
      driver?.vehiclePlate.trim() ?? '',
    ].where((value) => value.isNotEmpty).join(' • ');

    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            CircleAvatar(
              radius: 24,
              child: driverId != null || driver != null
                  ? Text(
                      name.isNotEmpty ? name.substring(0, 1).toUpperCase() : '?',
                      style: const TextStyle(fontWeight: FontWeight.bold),
                    )
                  : const Icon(Icons.person_off_outlined),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(displayName, style: theme.textTheme.titleSmall),
                  if (driver?.phone.trim().isNotEmpty == true) ...[
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Icon(Icons.phone, size: 15, color: theme.colorScheme.onSurfaceVariant),
                        const SizedBox(width: 5),
                        Text(driver!.phone, style: theme.textTheme.bodySmall),
                      ],
                    ),
                  ],
                  if (vehicle.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Icon(Icons.car_rental, size: 15, color: theme.colorScheme.onSurfaceVariant),
                        const SizedBox(width: 5),
                        Expanded(child: Text(vehicle, style: theme.textTheme.bodySmall)),
                      ],
                    ),
                  ],
                  if (driver == null) ...[
                    const SizedBox(height: 2),
                    Text(
                      driverId != null
                          ? 'Info nama/telepon/kendaraan belum tersedia di endpoint detail.'
                          : 'Ride sedang mencari driver terdekat.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// Route

class _RouteCard extends StatelessWidget {
  const _RouteCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            _detailRow(
              theme: theme,
              icon: Icons.arrow_upward,
              label: 'Pickup',
              value: order.pickupAddress.isNotEmpty
                  ? order.pickupAddress
                  : _coord(order.pickupLat, order.pickupLng),
            ),
            const SizedBox(height: 6),
            _detailRow(
              theme: theme,
              icon: Icons.arrow_downward,
              label: 'Dropoff',
              value: order.dropoffAddress.isNotEmpty
                  ? order.dropoffAddress
                  : _coord(order.dropoffLat, order.dropoffLng),
            ),
            const SizedBox(height: 6),
            _detailRow(
              theme: theme,
              icon: Icons.route_outlined,
              label: 'Jarak',
              value: '${order.distanceKm.toStringAsFixed(1)} km',
            ),
          ],
        ),
      ),
    );
  }
}

// Fare breakdown

class _FareCard extends StatelessWidget {
  const _FareCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(
              children: [
                Icon(Icons.receipt_long_outlined, size: 16, color: theme.colorScheme.onSurfaceVariant),
                const SizedBox(width: 8),
                Text('Rincian Biaya', style: theme.textTheme.titleSmall),
              ],
            ),
            const SizedBox(height: 10),
            _detailRow(
              theme: theme,
              icon: Icons.payments_outlined,
              label: 'Estimasi',
              value: _formatRupiah(order.estimatedFare),
            ),
            if (order.baseFare > 0)
              _detailRow(
                theme: theme,
                icon: Icons.explore_outlined,
                label: 'Dasar',
                value: _formatRupiah(order.baseFare),
              ),
            if (order.perKmRate > 0)
              _detailRow(
                theme: theme,
                icon: Icons.straighten,
                label: 'Per km',
                value: _formatRupiah(order.perKmRate),
              ),
            if (order.actualFare != null) ...[
              const Divider(height: 20),
              _detailRow(
                theme: theme,
                icon: Icons.check_circle_outline,
                label: 'Aktual',
                value: _formatRupiah(order.actualFare!),
              ),
            ],
            if (order.discountAmount != null && order.discountAmount! > 0)
              _detailRow(
                theme: theme,
                icon: Icons.percent,
                label: 'Diskon',
                value: '-${_formatRupiah(order.discountAmount!)}',
              ),
          ],
        ),
      ),
    );
  }
}

// Timestamps

class _TimelineCard extends StatelessWidget {
  const _TimelineCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(
              children: [
                Icon(Icons.schedule, size: 16, color: theme.colorScheme.onSurfaceVariant),
                const SizedBox(width: 8),
                Text('Waktu', style: theme.textTheme.titleSmall),
              ],
            ),
            const SizedBox(height: 10),
            _detailRow(
              theme: theme,
              icon: Icons.event,
              label: 'Dibuat',
              value: order.createdAt != null ? _formatDate(order.createdAt!) : '—',
            ),
            if (order.completedAt != null)
              _detailRow(
                theme: theme,
                icon: Icons.flag,
                label: 'Selesai',
                value: _formatDate(order.completedAt!),
              ),
            if (order.settledAt != null)
              _detailRow(
                theme: theme,
                icon: Icons.payments,
                label: 'Settle',
                value: _formatDate(order.settledAt!),
              ),
          ],
        ),
      ),
    );
  }
}

// Payment

class _PaymentCard extends StatelessWidget {
  const _PaymentCard({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(
              order.paymentMethod == 'CASH' ? Icons.money : Icons.account_balance_wallet_outlined,
              size: 20,
              color: theme.colorScheme.primary,
            ),
            const SizedBox(width: 10),
            Expanded(child: Text('Metode Pembayaran', style: theme.textTheme.bodyMedium)),
            Chip(
              label: Text(order.paymentMethod, style: const TextStyle(fontSize: 11)),
              visualDensity: VisualDensity.compact,
            ),
          ],
        ),
      ),
    );
  }
}

// Error & helper

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            const Text('Gagal memuat detail perjalanan', style: TextStyle(fontSize: 16)),
            const SizedBox(height: 8),
            Text(message, textAlign: TextAlign.center, style: const TextStyle(color: Colors.grey)),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Coba lagi'),
            ),
          ],
        ),
      ),
    );
  }
}

class _MissingArgsView extends StatelessWidget {
  const _MissingArgsView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Detail Perjalanan')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            const Text('Argumen order tidak ditemukan.'),
            const SizedBox(height: 8),
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Kembali'),
            ),
          ],
        ),
      ),
    );
  }
}

Widget _detailRow({
  required ThemeData theme,
  required IconData icon,
  required String label,
  required String value,
}) {
  return Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Icon(icon, size: 16, color: theme.colorScheme.onSurfaceVariant),
      const SizedBox(width: 8),
      SizedBox(width: 72, child: Text(label, style: theme.textTheme.bodySmall)),
      Expanded(
        child: Text(
          value,
          style: theme.textTheme.bodyMedium,
          overflow: TextOverflow.ellipsis,
        ),
      ),
    ],
  );
}

const Map<String, String> _statusText = {
  'SEARCHING_DRIVER': 'Mencari driver terdekat',
  'DRIVER_ASSIGNED': 'Driver menuju lokasi pickup',
  'DRIVER_ARRIVED': 'Driver menunggu di lokasi pickup',
  'TRIP_STARTED': 'Dalam perjalanan menuju tujuan',
  'COMPLETED': 'Perjalanan selesai',
  'SETTLED': 'Pembayaran selesai',
  'CANCELLED': 'Ride dibatalkan',
};

Color _statusColor(String status) {
  switch (status) {
    case 'COMPLETED':
    case 'SETTLED':
      return Colors.green.shade600;
    case 'CANCELLED':
      return Colors.red.shade600;
    case 'SEARCHING_DRIVER':
      return Colors.orange.shade600;
    default:
      return Colors.indigo.shade600;
  }
}

IconData _statusIcon(String status) {
  return switch (status) {
    'SEARCHING_DRIVER' => Icons.radar,
    'DRIVER_ASSIGNED' => Icons.person_pin_circle,
    'DRIVER_ARRIVED' => Icons.place,
    'TRIP_STARTED' => Icons.directions_car,
    'COMPLETED' => Icons.check_circle,
    'SETTLED' => Icons.payments,
    'CANCELLED' => Icons.cancel,
    _ => Icons.info_outline,
  };
}

String _shortId(String id) =>
    id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}

String _formatDate(DateTime dt) {
  final local = dt.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}/${two(local.month)}/${local.year} '
      '${two(local.hour)}:${two(local.minute)}';
}

String _coord(double lat, double lng) =>
    '${lat.toStringAsFixed(5)}, ${lng.toStringAsFixed(5)}';