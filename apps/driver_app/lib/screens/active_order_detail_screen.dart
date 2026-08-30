import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:latlong2/latlong.dart';

import '../config/constants.dart';
import '../config/router.dart';
import '../models/driver_order.dart';
import '../providers/order_provider.dart';
import '../widgets/kpi_card.dart';
import '../widgets/order_type_badge.dart';
import '../widgets/status_badge.dart';

class _StatusAction {
  const _StatusAction(this.label, this.target, {this.confirm});

  final String label;
  final String target;
  final String? confirm;
}

/// Active Order Detail (ROADMAP 3.10 A/B/C/D).
///
/// Menampilkan detail ride/food/send + map rute + tombol status sesuai state
/// machine masing-masing tipe. Untuk send multi-stop ada pintu ke
/// MultiStopDeliveryScreen.
class ActiveOrderDetailScreen extends ConsumerStatefulWidget {
  const ActiveOrderDetailScreen({
    super.key,
    required this.orderId,
    this.initialOrder,
  });

  final String orderId;
  final DriverOrder? initialOrder;

  @override
  ConsumerState<ActiveOrderDetailScreen> createState() => _ActiveOrderDetailScreenState();
}

class _ActiveOrderDetailScreenState extends ConsumerState<ActiveOrderDetailScreen> {
  late final MapController _mapController = MapController();
  bool _didFit = false;
  bool _updating = false;

  DriverOrder? get _order {
    final active = ref.read(activeOrdersProvider);
    for (final o in active.orders) {
      if (o.id == widget.orderId) return o;
    }
    return widget.initialOrder;
  }

  _StatusAction? _nextAction(DriverOrder order) {
    return switch (order.type) {
      OrderType.ride => switch (order.status) {
          'DRIVER_ASSIGNED' => const _StatusAction('Tiba di titik pickup', 'DRIVER_ARRIVED'),
          'DRIVER_ARRIVED' => const _StatusAction('Mulai perjalanan', 'TRIP_STARTED'),
          'TRIP_STARTED' => _StatusAction('Selesaikan perjalanan', 'COMPLETED',
              confirm: 'Konfirmasi selesai? Muatan akan otomatis di-settle.'),
          _ => null,
        },
      OrderType.food => switch (order.status) {
          'READY_FOR_PICKUP' => const _StatusAction('Ambil pesanan dari merchant', 'PICKED_UP'),
          'PICKED_UP' => const _StatusAction('Mulai mengantar', 'IN_TRANSIT'),
          'IN_TRANSIT' => _StatusAction('Tandai telah dikirim', 'DELIVERED',
              confirm: 'Konfirmasi order telah diterima customer? Settlement otomatis.'),
          _ => null,
        },
      OrderType.send => switch (order.status) {
          'DRIVER_ASSIGNED' => const _StatusAction('Ambil paket dari pengekspedisi', 'PICKED_UP'),
          'PICKED_UP' => const _StatusAction('Mulai mengantar', 'IN_TRANSIT'),
          'IN_TRANSIT' => null,
          _ => null,
        },
    };
  }

  Future<void> _applyAction(DriverOrder order, _StatusAction action) async {
    if (action.confirm != null) {
      final ok = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('Konfirmasi'),
          content: Text(action.confirm!),
          actions: [
            TextButton(onPressed: () => Navigator.of(dialogContext).pop(false), child: const Text('Batal')),
            FilledButton(onPressed: () => Navigator.of(dialogContext).pop(true), child: const Text('Ya')),
          ],
        ),
      );
      if (ok != true || !mounted) return;
    }
    setState(() => _updating = true);
    final success = await ref.read(activeOrdersProvider.notifier).updateStatus(order, action.target);
    if (!mounted) return;
    setState(() => _updating = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(success ? 'Status diperbarui: ${action.target.replaceAll('_', ' ')}' : 'Gagal update status.'),
      ),
    );
  }

  void _fitMap(DriverOrder order) {
    if (_didFit) return;
    _didFit = true;
    final pts = <LatLng>[
      LatLng(order.pickupLat, order.pickupLng),
      if (order.hasStops)
        ...order.stops.map((s) => LatLng(s.latitude, s.longitude))
      else if (order.deliveryLat != 0 || order.deliveryLng != 0)
        LatLng(order.deliveryLat, order.deliveryLng),
    ];
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || pts.isEmpty) return;
      _mapController.fitCamera(CameraFit.coordinates(coordinates: pts, padding: const EdgeInsets.all(48)));
    });
  }

  @override
  Widget build(BuildContext context) {
    final order = _order;
    if (order == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Detail Order')),
        body: const EmptyStateView(
          icon: Icons.search_off,
          message: 'Order tidak ditemukan di daftar aktif.\nKembali ke dashboard dan muat ulang.',
        ),
      );
    }
    _fitMap(order);
    final action = _nextAction(order);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text('${order.type.label} #${shortId(order.id)}'),
        actions: [
          if (order.isMock) const DemoBadge(tooltip: 'Mode demo: detail & update status disimulasikan di layanan local.'),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            flex: 3,
            child: FlutterMap(
              mapController: _mapController,
              options: MapOptions(
                initialCenter: LatLng(order.pickupLat, order.pickupLng),
                initialZoom: 12,
              ),
              children: [
                TileLayer(
                  urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                  userAgentPackageName: 'com.gflow.driver_app',
                ),
                PolylineLayer(
                  polylines: [
                    Polyline(
                      points: _routePoints(order),
                      color: Colors.indigo.withValues(alpha: 0.4),
                      strokeWidth: 4,
                    ),
                  ],
                ),
                MarkerLayer(markers: _routeMarkers(order)),
              ],
            ),
          ),
          Expanded(
            flex: 2,
            child: Material(
              color: Colors.white,
              elevation: 8,
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      children: [
                        OrderTypeBadge(type: order.type),
                        const SizedBox(width: 8),
                        StatusBadge(status: order.status),
                      ],
                    ),
                    const SizedBox(height: 12),
                    _detailRow(Icons.arrow_upward, 'Pickup', order.pickupAddress),
                    if (order.type == OrderType.food) ...[
                      _detailRow(Icons.storefront, 'Merchant', order.merchantName.isEmpty ? order.merchantAddress : '${order.merchantName} • ${order.merchantAddress}'),
                      _detailRow(Icons.arrow_downward, 'Kirim ke', order.deliveryAddress),
                    ] else if (order.type == OrderType.send) ...[
                      _detailRow(Icons.arrow_downward, 'Stops (${order.stopsCompleted}/${order.stops.length})', _stopsSummary(order)),
                    ] else
                      _detailRow(Icons.arrow_downward, 'Tujuan', order.deliveryAddress),
                    const Divider(height: 20),
                    Row(
                      children: [
                        Text('Earning', style: theme.textTheme.bodyMedium),
                        const Spacer(),
                        Text(
                          formatRupiah(order.driverEarning > 0 ? order.driverEarning : order.estimatedFare),
                          style: theme.textTheme.titleMedium?.copyWith(
                            color: Colors.green.shade800,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 14),
                    if (order.type == OrderType.send && order.status == 'IN_TRANSIT')
                      FilledButton.icon(
                        style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(46)),
                        onPressed: () => context.push(AppRoutes.multiStop(order.id), extra: order),
                        icon: const Icon(Icons.local_shipping),
                        label: const Text('Buka Multi-Stop Delivery'),
                      )
                    else if (action != null)
                      FilledButton.icon(
                        style: FilledButton.styleFrom(
                          minimumSize: const Size.fromHeight(46),
                          backgroundColor: theme.colorScheme.primary,
                        ),
                        onPressed: _updating ? null : () => _applyAction(order, action),
                        icon: _updating
                            ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Icon(Icons.arrow_forward),
                        label: Text(_updating ? 'Menyimpan…' : action.label),
                      )
                    else
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: Colors.grey.shade100,
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: Text(
                          order.isTerminal
                              ? 'Order sudah selesai/dibatalkan.'
                              : 'Tidak ada aksi untuk status ini.',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: Colors.grey.shade700, fontSize: 13),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  List<LatLng> _routePoints(DriverOrder order) {
    final pts = <LatLng>[
      LatLng(order.pickupLat, order.pickupLng),
    ];
    if (order.hasStops) {
      pts.addAll(order.stops.map((s) => LatLng(s.latitude, s.longitude)));
    } else if (order.deliveryLat != 0 || order.deliveryLng != 0) {
      pts.add(LatLng(order.deliveryLat, order.deliveryLng));
    }
    return pts;
  }

  List<Marker> _routeMarkers(DriverOrder order) {
    return [
      Marker(
        point: LatLng(order.pickupLat, order.pickupLng),
        width: 36,
        height: 36,
        child: const Icon(Icons.location_on, color: Colors.red, size: 34),
      ),
      if (order.type == OrderType.send)
        for (final s in order.stops)
          Marker(
            point: LatLng(s.latitude, s.longitude),
            width: 30,
            height: 30,
            child: Icon(
              s.isDelivered ? Icons.check_circle : Icons.place,
              color: s.isDelivered ? Colors.green : Colors.orange,
              size: 28,
            ),
          )
      else if (order.deliveryLat != 0 || order.deliveryLng != 0)
        Marker(
          point: LatLng(order.deliveryLat, order.deliveryLng),
          width: 32,
          height: 32,
          child: const Icon(Icons.location_on, color: Colors.green, size: 32),
        ),
    ];
  }

  static String _stopsSummary(DriverOrder order) {
    final next = order.nextUndeliveredStop;
    if (next == null) return 'Semua stop selesai';
    final done = order.stopsCompleted;
    return 'Stop ${done + 1}/${order.stops.length}: ${next.address.isNotEmpty ? next.address : 'belum diketahui'}';
  }
}

Widget _detailRow(IconData icon, String label, String value) {
  return Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 16, color: Colors.grey),
        const SizedBox(width: 8),
        SizedBox(width: 72, child: Text(label, style: const TextStyle(fontSize: 12, color: Colors.grey))),
        Expanded(
          child: Text(
            value,
            style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w500),
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    ),
  );
}