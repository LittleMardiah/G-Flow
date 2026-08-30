import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:url_launcher/url_launcher.dart';

import '../config/constants.dart';
import '../models/driver_order.dart';
import '../models/driver_stop.dart';
import '../providers/order_provider.dart';
import '../widgets/kpi_card.dart';
import '../widgets/order_type_badge.dart';
import '../widgets/status_badge.dart';

/// Multi-Stop Send Delivery (ROADMAP 3.10 D/E).
///
/// Menampilkan semua stop secara berurutan: alamat, nama/telepon penerima,
/// allocated fare, status PENDING → COMPLETED. Driver bisa "Navigate ke
/// stop berikutnya" (Google Maps) dan menandai stop delivered (foto opsional
/// via image_picker). Stop terakhir terselesaikan → order otomatis DELIVERED.
class MultiStopDeliveryScreen extends ConsumerStatefulWidget {
  const MultiStopDeliveryScreen({
    super.key,
    required this.orderId,
    this.initialOrder,
  });

  final String orderId;
  final DriverOrder? initialOrder;

  @override
  ConsumerState<MultiStopDeliveryScreen> createState() => _MultiStopDeliveryScreenState();
}

class _MultiStopDeliveryScreenState extends ConsumerState<MultiStopDeliveryScreen> {
  final ImagePicker _picker = ImagePicker();
  bool _submitting = false;

  DriverOrder? get _order {
    final active = ref.read(activeOrdersProvider);
    for (final o in active.orders) {
      if (o.id == widget.orderId) return o;
    }
    return widget.initialOrder;
  }

  Future<void> _navigate(DriverStop stop) async {
    final uri = Uri.parse(
      'https://www.google.com/maps/dir/?api=1&destination=${stop.latitude},${stop.longitude}',
    );
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Tidak bisa membuka Google Maps')),
      );
    }
  }

  Future<String?> _pickPhoto() async {
    final file = await _picker.pickImage(source: ImageSource.camera, maxWidth: 1024);
    if (file == null) return null;
    final bytes = await file.readAsBytes();
    return base64Encode(bytes);
  }

  Future<void> _markDelivered(DriverOrder order, DriverStop stop) async {
    final photo = await _pickPhoto();
    if (!mounted) return;
    var usePhoto = photo != null;
    if (usePhoto) {
      final confirm = await showDialog<bool>(
            context: context,
            builder: (dialogContext) => AlertDialog(
              title: const Text('Kirim bukti foto?'),
              actions: [
                TextButton(onPressed: () => Navigator.of(dialogContext).pop(false), child: const Text('Tanpa foto')),
                FilledButton(onPressed: () => Navigator.of(dialogContext).pop(true), child: const Text('Dengan foto')),
              ],
            ),
          ) ??
          false;
      if (!mounted) return;
      usePhoto = confirm;
    }

    setState(() => _submitting = true);
    final ok = await ref.read(activeOrdersProvider.notifier).markStopDelivered(
          order,
          stop.id,
          deliveryPhotoUrl: usePhoto ? photo : null,
        );
    if (!mounted) return;
    setState(() => _submitting = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(ok
            ? 'Stop ${stop.stopNumber} ditandai DELIVERED.'
            : 'Gagal update stop: kapasitas penuh?'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final order = _order;
    if (order == null || !order.hasStops) {
      return Scaffold(
        appBar: AppBar(title: const Text('Multi-Stop')),
        body: const EmptyStateView(
          icon: Icons.local_shipping,
          message: 'Order send tidak ditemukan atau belum memiliki stop.',
        ),
      );
    }

    final sorted = List<DriverStop>.from(order.stops)
      ..sort((a, b) => a.stopNumber.compareTo(b.stopNumber));
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text('Send #${shortId(order.id)}'),
        actions: [
          if (order.isMock) const DemoBadge(tooltip: 'Mode demo: update stop disimulasikan di layanan local.'),
        ],
      ),
      body: Column(
        children: [
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            color: theme.colorScheme.primaryContainer.withValues(alpha: 0.4),
            child: Row(
              children: [
                OrderTypeBadge(type: order.type),
                const SizedBox(width: 8),
                StatusBadge(status: order.status),
                const Spacer(),
                Text(
                  '${order.stopsCompleted}/${order.stops.length} selesai',
                  style: theme.textTheme.labelMedium,
                ),
              ],
            ),
          ),
          const SizedBox(height: 6),
          LinearProgressIndicator(
            value: order.stops.isEmpty ? 0 : order.stopsCompleted / order.stops.length,
            minHeight: 5,
            backgroundColor: theme.colorScheme.surfaceContainerHighest,
          ),
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: sorted.length,
              itemBuilder: (context, i) {
                final stop = sorted[i];
                final isNext = stop == order.nextUndeliveredStop;
                return _StopCard(
                  stop: stop,
                  isNext: isNext,
                  allDone: order.isTerminal,
                  submitting: _submitting,
                  onNavigate: () => _navigate(stop),
                  onDeliver: () => _markDelivered(order, stop),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _StopCard extends StatelessWidget {
  const _StopCard({
    required this.stop,
    required this.isNext,
    required this.allDone,
    required this.submitting,
    required this.onNavigate,
    required this.onDeliver,
  });

  final DriverStop stop;
  final bool isNext;
  final bool allDone;
  final bool submitting;
  final VoidCallback onNavigate;
  final VoidCallback onDeliver;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final delivered = stop.isDelivered;
    final borderColor = delivered
        ? Colors.green
        : isNext
            ? theme.colorScheme.primary
            : theme.colorScheme.outlineVariant;

    return Card(
      margin: const EdgeInsets.symmetric(vertical: 6),
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: borderColor, width: isNext ? 1.5 : 1),
      ),
      color: delivered ? Colors.green.shade50 : theme.colorScheme.surfaceContainerLow,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 14,
                  backgroundColor: delivered ? Colors.green : (isNext ? theme.colorScheme.primary : Colors.grey),
                  child: Text(
                    '${stop.stopNumber}',
                    style: const TextStyle(color: Colors.white, fontSize: 13, fontWeight: FontWeight.bold),
                  ),
                ),
                const SizedBox(width: 8),
                Text('Stop ${stop.stopNumber}', style: theme.textTheme.titleSmall),
                const Spacer(),
                StatusBadge(status: delivered ? 'COMPLETED' : 'PENDING'),
              ],
            ),
            const SizedBox(height: 10),
            _line(Icons.place_outlined, stop.address.isEmpty ? 'Alamat tidak tersedia' : stop.address),
            if (stop.recipientName.isNotEmpty || stop.recipientPhone.isNotEmpty)
              _line(
                Icons.person_outline,
                [stop.recipientName, stop.recipientPhone].where((e) => e.isNotEmpty).join(' • '),
              ),
            _line(Icons.savings_outlined, 'Fare stop: ${formatRupiah(stop.allocatedFare)}'),
            if (stop.distanceKm > 0)
              _line(Icons.route_outlined, 'Jarak stop: ${stop.distanceKm.toStringAsFixed(2)} km'),
            if (!delivered && isNext) ...[
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: submitting ? null : onNavigate,
                      icon: const Icon(Icons.navigation_outlined, size: 18),
                      label: const Text('Navigasi'),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: FilledButton.icon(
                      onPressed: submitting ? null : onDeliver,
                      icon: const Icon(Icons.check_circle_outline, size: 18),
                      label: Text(submitting ? '…' : 'Sudah Sampai'),
                    ),
                  ),
                ],
              ),
            ],
            if (allDone && !delivered)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  'Order sudah selesai.',
                  style: TextStyle(color: Colors.grey.shade600, fontSize: 12),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _line(IconData icon, String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 15, color: Colors.grey),
          const SizedBox(width: 6),
          Expanded(child: Text(text, style: const TextStyle(fontSize: 13), overflow: TextOverflow.ellipsis)),
        ],
      ),
    );
  }
}