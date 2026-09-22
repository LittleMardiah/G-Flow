import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:latlong2/latlong.dart';

import '../../config/router.dart';
import '../../models/ride_order.dart';
import '../../providers/ride_provider.dart';

/// Ride Tracking Screen (ROADMAP 02 bagian 3.2 & HALAMAN.txt 3.3).
///
/// Fitur:
///  - Peta live (flutter_map/OpenStreetMap — konsisten dengan booking screen,
///    tanpa API key) dengan marker pickup, dropoff, dan marker driver bergerak.
///  - Status timeline (stepper): SEARCHING_DRIVER → DRIVER_ASSIGNED →
///    DRIVER_ARRIVED → TRIP_STARTED → COMPLETED.
///  - Kartu info driver (nama, rating, kendaraan, tombol Hubungi/Chat).
///  - Tombol Cancel Ride dengan konfirmasi (fee sesuai status).
///  - Polling status setiap 3 detik lewat `rideTrackingProvider`.
///  - Handling loading / network error / empty state.
///
/// Argumen navigasi: `RideTrackingArgs` (dikirim via Navigator.pushNamed).
class RideTrackingScreen extends ConsumerStatefulWidget {
  const RideTrackingScreen({super.key});

  @override
  ConsumerState<RideTrackingScreen> createState() => _RideTrackingScreenState();
}

class _RideTrackingScreenState extends ConsumerState<RideTrackingScreen> {
  final MapController _mapController = MapController();
  bool _didFitCamera = false;

  RideTrackingArgs? get _args {
    final raw = ModalRoute.of(context)?.settings.arguments;
    return raw is RideTrackingArgs ? raw : null;
  }

  @override
  Widget build(BuildContext context) {
    final args = _args;
    if (args == null) return const _MissingArgsView();

    final tracking = ref.watch(rideTrackingProvider(args));

    // Fit peta ke rute (pickup → dropoff) saat data order pertama kali ada.
    ref.listen<RideTrackingState>(rideTrackingProvider(args), (prev, next) {
      if (!_didFitCamera && next.order != null && prev?.order == null) {
        _didFitCamera = true;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          final order = next.order;
          if (order == null) return;
          _mapController.fitCamera(CameraFit.coordinates(
            coordinates: [order.pickup, order.dropoff],
            padding: const EdgeInsets.all(56),
          ));
        });
      }
    });

    return Scaffold(
      appBar: AppBar(
        title: Text('Ride #${_shortId(args.orderId)}'),
        actions: [
          if (tracking.isMock) const _DemoBadge(),
        ],
      ),
      body: _buildBody(context, args, tracking),
    );
  }

  Widget _buildBody(
    BuildContext context,
    RideTrackingArgs args,
    RideTrackingState tracking,
  ) {
    final order = tracking.order;

    // Empty/Loading state (belum ada data order sama sekali).
    if (order == null) {
      if (tracking.error != null) {
        return _ErrorView(
          message: tracking.error!,
          onRetry: () => ref.read(rideTrackingProvider(args).notifier).retry(),
        );
      }
      return const Center(child: CircularProgressIndicator());
    }

    return Column(
      children: [
        // Error banner: polling sempat gagal tapi masih ada data terakhir.
        if (tracking.error != null)
          Container(
            width: double.infinity,
            color: Colors.red.shade50,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
            child: Text(
              tracking.error!,
              style: TextStyle(color: Colors.red.shade800, fontSize: 12),
            ),
          ),
        Expanded(flex: 3, child: _buildMap(tracking)),
        Expanded(flex: 2, child: _buildInfoPanel(context, args, tracking)),
      ],
    );
  }

  Widget _buildMap(RideTrackingState tracking) {
    final order = tracking.order!;
    final driverLoc = tracking.driverLocation;

    return FlutterMap(
      mapController: _mapController,
      options: MapOptions(
        initialCenter: order.pickup,
        initialZoom: 13,
        interactionOptions: const InteractionOptions(
          flags: InteractiveFlag.all & ~InteractiveFlag.rotate,
        ),
      ),
      children: [
        TileLayer(
          urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
          userAgentPackageName: 'com.gflow.customer_app',
        ),
        PolylineLayer(
          polylines: [
            Polyline(
              points: [order.pickup, order.dropoff],
              color: Colors.blue.withValues(alpha: 0.35),
              strokeWidth: 4,
            ),
          ],
        ),
        MarkerLayer(
          markers: [
            Marker(
              point: order.pickup,
              width: 40,
              height: 40,
              child: const Icon(Icons.location_pin, color: Colors.red, size: 40),
            ),
            Marker(
              point: order.dropoff,
              width: 40,
              height: 40,
              child: const Icon(Icons.place, color: Colors.green, size: 36),
            ),
            if (driverLoc != null)
              Marker(
                point: driverLoc,
                width: 44,
                height: 44,
                child: Container(
                  decoration: const BoxDecoration(
                    color: Colors.white,
                    shape: BoxShape.circle,
                  ),
                  padding: const EdgeInsets.all(2),
                  child: const Icon(Icons.directions_car, color: Colors.indigo, size: 34),
                ),
              ),
          ],
        ),
      ],
    );
  }

  Widget _buildInfoPanel(
    BuildContext context,
    RideTrackingArgs args,
    RideTrackingState tracking,
  ) {
    final order = tracking.order!;
    final theme = Theme.of(context);

    return Material(
      color: Colors.white,
      elevation: 8,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _StatusCard(order: order, driverLocation: tracking.driverLocation),
            const SizedBox(height: 12),
            if (tracking.driver != null) ...[
              _DriverCard(
                driver: tracking.driver!,
                onCall: () => _notImplemented('Hubungi driver'),
                onMessage: () => _notImplemented('Chat driver'),
              ),
              const SizedBox(height: 12),
            ] else
              const _SearchingCard(),
            _OrderDetailsCard(order: order),
            const SizedBox(height: 16),
            if (order.isTerminal)
              _TerminalBox(
                order: order,
                onDone: () => Navigator.of(context).pop(),
                onViewDetail: _detailEnabled(order)
                    ? () => Navigator.of(context).pushNamed(
                          AppRoutes.rideDetail,
                          arguments: RideDetailArgs(orderId: order.id),
                        )
                    : null,
              )
            else
              _CancelButton(
                isCancelling: tracking.isCancelling,
                onPressed: () => _confirmCancel(context, args, tracking),
              ),
            const SizedBox(height: 8),
            if (tracking.message != null)
              Text(
                tracking.message!,
                textAlign: TextAlign.center,
                style: theme.textTheme.bodySmall?.copyWith(color: theme.colorScheme.primary),
              ),
          ],
        ),
      ),
    );
  }

  void _notImplemented(String feature) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('$feature: fitur placeholder (TODO: integrasi lanjutan).')),
    );
  }

  Future<void> _confirmCancel(
    BuildContext context,
    RideTrackingArgs args,
    RideTrackingState tracking,
  ) async {
    final order = tracking.order;
    final fee = switch (order?.status) {
      'DRIVER_ASSIGNED' => 5000,
      'DRIVER_ARRIVED' => 10000,
      _ => 0,
    };
    final feeText = fee > 0
        ? 'Anda akan dikenakan biaya pembatalan ${_formatRupiah(fee)}.'
        : 'Pembatalan saat ini tidak dikenakan biaya.';

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Batalkan ride?'),
        content: Text(
          '$feeText\n\nDana escrow (jika pembayaran WALLET) akan dikembalikan '
          'sesuai aturan state machine.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Kembali'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: Colors.red.shade700,
            ),
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Ya, batalkan'),
          ),
        ],
      ),
    );

    if (confirmed != true || !context.mounted) return;

    final success = await ref.read(rideTrackingProvider(args).notifier).cancelRide();
    if (success && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Ride berhasil dibatalkan')),
      );
    }
  }

  static String _shortId(String id) =>
      id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();
}

// Status (stepper)

class _StatusCard extends StatelessWidget {
  const _StatusCard({required this.order, this.driverLocation});

  final RideOrder order;
  final LatLng? driverLocation;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.55),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _StatusStepper(status: order.status),
            const SizedBox(height: 16),
            Row(
              children: [
                Icon(_statusIcon(order.status), size: 20, color: theme.colorScheme.primary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    _statusLabel(order.status),
                    style: theme.textTheme.titleSmall,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              _etaText(order, driverLocation),
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusStepper extends StatelessWidget {
  const _StatusStepper({required this.status});

  final String status;

  static const List<({String label, IconData icon})> _steps = [
    (label: 'Mencari', icon: Icons.search),
    (label: 'Ditemukan', icon: Icons.person_pin_circle),
    (label: 'Tiba', icon: Icons.place),
    (label: 'Berjalan', icon: Icons.directions_car),
    (label: 'Selesai', icon: Icons.check_circle),
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currentIndex = kRideStatusFlow.indexOf(status);
    final done = currentIndex < 0 ? -1 : currentIndex;
    final active = theme.colorScheme.primary;
    final inactive = theme.colorScheme.surfaceContainerHighest;
    final outline = theme.colorScheme.outline;

    return Row(
      children: [
        for (var i = 0; i < _steps.length; i++) ...[
          Expanded(
            flex: 2,
            child: _step(
              step: _steps[i],
              reached: i < done,
              isCurrent: i == done,
              active: active,
              inactive: inactive,
              outline: outline,
              doneColor: (done >= 0 && i == done)
                  ? (status == 'COMPLETED' || status == 'SETTLED'
                      ? Colors.green
                      : active)
                  : active,
            ),
          ),
          if (i < _steps.length - 1)
            Expanded(
              flex: 1,
              child: Container(
                height: 2,
                margin: const EdgeInsets.only(bottom: 14),
                color: i < done ? active : inactive,
              ),
            ),
        ],
      ],
    );
  }

  Widget _step({
    required ({String label, IconData icon}) step,
    required bool reached,
    required bool isCurrent,
    required Color active,
    required Color inactive,
    required Color outline,
    required Color doneColor,
  }) {
    final Color bg;
    final Color fg;
    final IconData icon;
    if (reached) {
      bg = doneColor;
      fg = Colors.white;
      icon = Icons.check;
    } else if (isCurrent) {
      bg = Colors.white;
      fg = active;
      icon = step.icon;
    } else {
      bg = inactive;
      fg = outline;
      icon = step.icon;
    }

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        AnimatedContainer(
          duration: const Duration(milliseconds: 300),
          width: 30,
          height: 30,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: bg,
            border: Border.all(
              color: isCurrent ? active : Colors.transparent,
              width: 2.5,
            ),
          ),
          child: Icon(icon, color: fg, size: 16),
        ),
        const SizedBox(height: 4),
        Text(
          step.label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(
            fontSize: 9,
            fontWeight: isCurrent ? FontWeight.bold : FontWeight.normal,
            color: reached ? active : outline,
          ),
        ),
      ],
    );
  }
}

// Kartu driver

class _DriverCard extends StatelessWidget {
  const _DriverCard({
    required this.driver,
    required this.onCall,
    required this.onMessage,
  });

  final DriverInfo driver;
  final VoidCallback onCall;
  final VoidCallback onMessage;

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
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 24,
                  child: Text(
                    _initials(driver.name),
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(driver.name, style: theme.textTheme.titleSmall),
                      const SizedBox(height: 2),
                      Row(
                        children: [
                          const Icon(Icons.star, color: Colors.amber, size: 16),
                          const SizedBox(width: 2),
                          Text(
                            driver.rating.toStringAsFixed(1),
                            style: theme.textTheme.bodySmall,
                          ),
                          if (driver.reviewCount > 0) ...[
                            const SizedBox(width: 6),
                            Text(
                              '(${driver.reviewCount})',
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: theme.colorScheme.onSurfaceVariant,
                              ),
                            ),
                          ],
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Icon(Icons.car_rental, size: 16, color: theme.colorScheme.onSurfaceVariant),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    '${driver.vehicleType} • ${driver.vehiclePlate}',
                    style: theme.textTheme.bodyMedium,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: onCall,
                    icon: const Icon(Icons.call, size: 18),
                    label: const Text('Hubungi'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: onMessage,
                    icon: const Icon(Icons.chat_bubble_outline, size: 18),
                    label: const Text('Chat'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SearchingCard extends StatelessWidget {
  const _SearchingCard();

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
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.radar, size: 22),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Mencari driver terdekat…', style: theme.textTheme.titleSmall),
                      const SizedBox(height: 2),
                      Text(
                        'Driver terdekat akan menerima pesanan Anda.',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            const LinearProgressIndicator(),
          ],
        ),
      ),
    );
  }
}

// Rincian order

class _OrderDetailsCard extends StatelessWidget {
  const _OrderDetailsCard({required this.order});

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
            const Divider(height: 20),
            Row(
              children: [
                Icon(Icons.payments_outlined, size: 16, color: theme.colorScheme.onSurfaceVariant),
                const SizedBox(width: 8),
                Text('Fare', style: theme.textTheme.bodySmall),
                const SizedBox(width: 6),
                Chip(
                  label: Text(
                    order.paymentMethod,
                    style: const TextStyle(fontSize: 10),
                  ),
                  visualDensity: VisualDensity.compact,
                ),
                const Spacer(),
                Flexible(
                  child: Text(
                    _formatRupiah(order.estimatedFare),
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
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
        SizedBox(
          width: 64,
          child: Text(label, style: theme.textTheme.bodySmall),
        ),
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
}

// Aksi terminal & cancel

class _TerminalBox extends StatelessWidget {
  const _TerminalBox({required this.order, required this.onDone, this.onViewDetail});

  final RideOrder order;
  final VoidCallback onDone;
  final VoidCallback? onViewDetail;

  @override
  Widget build(BuildContext context) {
    final done = order.status == 'COMPLETED' || order.status == 'SETTLED';
    final color = done ? Colors.green.shade600 : Colors.red.shade600;
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.1),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: color),
          ),
          child: Row(
            children: [
              Icon(done ? Icons.check_circle : Icons.cancel, color: color),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  done ? 'Ride selesai. Terima kasih!' : 'Ride telah dibatalkan.',
                  style: TextStyle(color: color, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        if (done && onViewDetail != null) ...[
          FilledButton.icon(
            onPressed: onViewDetail,
            icon: const Icon(Icons.receipt_long_outlined),
            label: const Text('Lihat Detail'),
          ),
          const SizedBox(height: 8),
        ],
        OutlinedButton.icon(
          onPressed: onDone,
          icon: const Icon(Icons.home_outlined),
          label: const Text('Kembali ke Beranda'),
        ),
      ],
    );
  }
}

class _CancelButton extends StatelessWidget {
  const _CancelButton({required this.isCancelling, required this.onPressed});

  final bool isCancelling;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final color = Colors.red.shade700;
    return OutlinedButton.icon(
      style: OutlinedButton.styleFrom(
        foregroundColor: color,
        minimumSize: const Size.fromHeight(48),
        side: BorderSide(color: color),
      ),
      onPressed: isCancelling ? null : onPressed,
      icon: isCancelling
          ? const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : const Icon(Icons.cancel_outlined),
      label: Text(isCancelling ? 'Membatalkan…' : 'Cancel Ride'),
    );
  }
}

// Error & helper view

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
            const Text('Gagal memuat status ride', style: TextStyle(fontSize: 16)),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.grey),
            ),
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
      appBar: AppBar(title: const Text('Ride Tracking')),
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

class _DemoBadge extends StatelessWidget {
  const _DemoBadge();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.only(right: 12),
        child: Tooltip(
          message:
              'Mode demo: endpoint GET /rides/{id} & lokasi driver belum ada '
              'di backend (lihat kUseMockRideData di lib/services/ride_service.dart).',
          child: const Text(
            'DEMO',
            style: TextStyle(
              color: Colors.orange,
              fontSize: 12,
              fontWeight: FontWeight.bold,
              letterSpacing: 1.2,
            ),
          ),
        ),
      ),
    );
  }
}

// Helper murni

const Map<String, String> _statusText = {
  'SEARCHING_DRIVER': 'Mencari driver',
  'DRIVER_ASSIGNED': 'Driver menuju lokasi pickup',
  'DRIVER_ARRIVED': 'Driver menunggu di lokasi pickup',
  'TRIP_STARTED': 'Dalam perjalanan menuju tujuan',
  'COMPLETED': 'Perjalanan selesai',
  'SETTLED': 'Pembayaran selesai',
  'CANCELLED': 'Ride dibatalkan',
};

String _statusLabel(String status) => _statusText[status] ?? status.replaceAll('_', ' ');

bool _detailEnabled(RideOrder order) =>
    order.status == 'COMPLETED' || order.status == 'SETTLED';

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

/// ETA countdown sederhana berdasarkan simulasi posisi driver (HALAMAN 3.3).
String _etaText(RideOrder order, LatLng? driverLocation) {
  switch (order.status) {
    case 'SEARCHING_DRIVER':
      return 'Mencari driver terdekat…';
    case 'DRIVER_ASSIGNED':
      if (driverLocation == null) return 'Menunggu pembaruan posisi driver…';
      final minutes = math.max(1, (_distanceKm(driverLocation, order.pickup) / 25 * 60).ceil());
      return '$minutes menit menuju lokasi pickup';
    case 'DRIVER_ARRIVED':
      return 'Driver sudah menunggu di lokasi pickup';
    case 'TRIP_STARTED':
      if (driverLocation == null) return 'Dalam perjalanan…';
      final minutes = math.max(1, (_distanceKm(driverLocation, order.dropoff) / 30 * 60).ceil());
      return '$minutes menit menuju tujuan';
    case 'COMPLETED':
      return 'Perjalanan telah selesai';
    case 'SETTLED':
      return 'Pembayaran telah diselesaikan';
    case 'CANCELLED':
      return order.cancellationReason == null
          ? 'Ride dibatalkan'
          : 'Ride dibatalkan (${order.cancellationReason})';
    default:
      return '';
  }
}

double _distanceKm(LatLng a, LatLng b) {
  const earthRadiusKm = 6371.0;
  double rad(double deg) => deg * math.pi / 180;
  final dLat = rad(b.latitude - a.latitude);
  final dLng = rad(b.longitude - a.longitude);
  final h = math.pow(math.sin(dLat / 2), 2) +
      math.cos(rad(a.latitude)) * math.cos(rad(b.latitude)) * math.pow(math.sin(dLng / 2), 2);
  return 2 * earthRadiusKm * math.asin(math.sqrt(h));
}

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}

String _coord(double lat, double lng) =>
    '${lat.toStringAsFixed(5)}, ${lng.toStringAsFixed(5)}';

String _initials(String name) {
  final parts = name.trim().split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
  if (parts.isEmpty) return '?';
  final first = parts.first.substring(0, 1).toUpperCase();
  if (parts.length == 1) return first;
  return first + parts.last.substring(0, 1).toUpperCase();
}