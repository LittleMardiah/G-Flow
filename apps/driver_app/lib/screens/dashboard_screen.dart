import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:latlong2/latlong.dart';

import '../config/constants.dart';
import '../config/router.dart';
import '../models/driver_order.dart';
import '../providers/auth_provider.dart';
import '../providers/earnings_provider.dart';
import '../providers/location_provider.dart';
import '../providers/order_provider.dart';
import '../widgets/kpi_card.dart';
import '../widgets/order_type_badge.dart';
import '../widgets/status_badge.dart';

/// Multi-Order Dashboard (ROADMAP 3.10 A).
///
/// Menampilkan: toggle online/offline, peta lokasi + pin order, ringkasan
/// earnings hari ini, indikator kapasitas (maks 3), dan daftar order aktif
/// yang dikelompokkan per tipe: Rides/Food/Send.
class DashboardScreen extends ConsumerStatefulWidget {
  const DashboardScreen({super.key});

  @override
  ConsumerState<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends ConsumerState<DashboardScreen> {
  final MapController _mapController = MapController();
  bool _didFitCamera = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _bootstrap());
  }

  Future<void> _bootstrap() async {
    final driverId = await ref.read(currentDriverIdProvider.future);
    if (!mounted) return;
    final active = ref.read(activeOrdersProvider.notifier);
    active.setDriverId(driverId);
    if (driverId != null && driverId.isNotEmpty) {
      ref.read(earningsProvider.notifier).fetch(driverId);
    }
    final loc = ref.read(driverLocationProvider.notifier);
    await loc.updateNow();
    if (!mounted) return;
    _fitCamera();
  }

  void _fitCamera() {
    if (_didFitCamera) return;
    _didFitCamera = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final pos = ref.read(driverLocationProvider);
      _mapController.move(LatLng(pos.latitude, pos.longitude), 12);
    });
  }

  @override
  Widget build(BuildContext context) {
    final location = ref.watch(driverLocationProvider);
    final active = ref.watch(activeOrdersProvider);
    final earnings = ref.watch(earningsProvider);

    final types = <OrderType, List<DriverOrder>>{};
    for (final o in active.orders) {
      types.putIfAbsent(o.type, () => []).add(o);
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(kAppName),
        actions: [
          if (active.orders.any((o) => o.isMock)) const DemoBadge(tooltip: 'Order aktif DEMO: endpoint daftar order driver belum ada di backend.'),
          IconButton(
            tooltip: 'Logout',
            icon: const Icon(Icons.logout),
            onPressed: () => ref.read(authProvider.notifier).logout(),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.read(activeOrdersProvider.notifier).fetch();
          final id = await ref.read(currentDriverIdProvider.future);
          if (id != null && id.isNotEmpty) {
            ref.read(earningsProvider.notifier).fetch(id);
          }
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _OnlineToggle(
              online: location.isOnline,
              isPublishing: location.isPublishing,
              onChanged: (v) => ref.read(driverLocationProvider.notifier).setOnline(v),
            ),
            const SizedBox(height: 12),
            _DriverMap(
              controller: _mapController,
              location: location,
              activeOrders: active.orders,
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: KpiCard(
                    icon: Icons.payments_outlined,
                    label: 'Earnings hari ini',
                    value: formatRupiah(earnings.earning?.todayTotal ?? 0),
                    iconColor: Colors.green,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: KpiCard(
                    icon: Icons.route_outlined,
                    label: 'Order hari ini',
                    value: '${earnings.earning?.todayOrderCount ?? 0}',
                    iconColor: Colors.indigo,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            _CapacityCard(
              activeCount: active.activeCount,
              maxActive: kMaxActiveOrders,
              showWarning: active.isAtCapacity,
            ),
            if (active.error != null)
              _ErrorBanner(message: active.error!),
            const SizedBox(height: 12),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('Order Aktif', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold)),
                TextButton(
                  onPressed: () => context.go(AppRoutes.availableOrders),
                  child: const Text('Lihat pesanan tersedia'),
                ),
              ],
            ),
            if (active.isLoading)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 32),
                child: Center(child: CircularProgressIndicator()),
              )
            else if (active.orders.isEmpty)
              const EmptyStateView(
                icon: Icons.local_shipping,
                message: 'Belum ada order aktif.\nTap "Pesanan tersedia" untuk accept order baru.',
              )
            else
              for (final entry in types.entries) ...[
                _GroupHeader(type: entry.key, count: entry.value.length),
                for (final order in entry.value) _ActiveOrderCard(order: order),
                const SizedBox(height: 4),
              ],
            const SizedBox(height: 80),
          ],
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        heroTag: 'available-orders',
        onPressed: () => context.go(AppRoutes.availableOrders),
        icon: const Icon(Icons.add),
        label: const Text('Pesanan Tersedia'),
      ),
    );
  }
}

class _OnlineToggle extends StatelessWidget {
  const _OnlineToggle({
    required this.online,
    required this.isPublishing,
    required this.onChanged,
  });

  final bool online;
  final bool isPublishing;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    final color = online ? Colors.green.shade600 : Colors.grey;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 8),
          Text(
            online ? 'ONLINE' : 'OFFLINE',
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.bold,
              letterSpacing: 0.8,
            ),
          ),
          const Spacer(),
          if (isPublishing)
            const SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          Switch(value: online, onChanged: onChanged),
        ],
      ),
    );
  }
}

class _DriverMap extends StatelessWidget {
  const _DriverMap({
    required this.controller,
    required this.location,
    required this.activeOrders,
  });

  final MapController controller;
  final DriverLocationState location;
  final List<DriverOrder> activeOrders;

  @override
  Widget build(BuildContext context) {
    final myPos = LatLng(location.latitude, location.longitude);
    return ClipRRect(
      borderRadius: BorderRadius.circular(14),
      child: SizedBox(
        height: 180,
        child: FlutterMap(
          mapController: controller,
          options: MapOptions(
            initialCenter: myPos,
            initialZoom: 12,
            interactionOptions: const InteractionOptions(
              flags: InteractiveFlag.all & ~InteractiveFlag.rotate,
            ),
          ),
          children: [
            TileLayer(
              urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
              userAgentPackageName: 'com.gflow.driver_app',
            ),
            MarkerLayer(
              markers: [
                Marker(
                  point: myPos,
                  width: 36,
                  height: 36,
                  child: const Center(
                    child: Icon(Icons.my_location, color: Colors.indigo, size: 30),
                  ),
                ),
                for (final o in activeOrders)
                  Marker(
                    point: _destPoint(o),
                    width: 34,
                    height: 34,
                    child: Center(
                      child: Icon(
                        _typeIcon(o.type),
                        color: _typeColor(o.type),
                        size: 26,
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  static LatLng _destPoint(DriverOrder o) => LatLng(o.deliveryLat, o.deliveryLng);
  static IconData _typeIcon(OrderType t) => switch (t) {
        OrderType.ride => Icons.directions_car,
        OrderType.food => Icons.restaurant,
        OrderType.send => Icons.inventory_2,
      };
  static Color _typeColor(OrderType t) => switch (t) {
        OrderType.ride => Colors.blue,
        OrderType.food => Colors.orange,
        OrderType.send => Colors.green,
      };
}

class _CapacityCard extends StatelessWidget {
  const _CapacityCard({
    required this.activeCount,
    required this.maxActive,
    required this.showWarning,
  });

  final int activeCount;
  final int maxActive;
  final bool showWarning;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final ratio = maxActive == 0 ? 0.0 : activeCount / maxActive;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: showWarning ? Colors.orange : theme.colorScheme.outlineVariant,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.speed,
            color: showWarning ? Colors.orange : theme.colorScheme.primary,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Kapasitas: $activeCount/$maxActive',
                    style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.bold)),
                const SizedBox(height: 6),
                ClipRRect(
                  borderRadius: BorderRadius.circular(4),
                  child: LinearProgressIndicator(
                    value: ratio,
                    minHeight: 6,
                    color: showWarning ? Colors.orange : theme.colorScheme.primary,
                    backgroundColor: theme.colorScheme.surfaceContainerHighest,
                  ),
                ),
              ],
            ),
          ),
          if (showWarning)
            const Padding(
              padding: EdgeInsets.only(left: 10),
              child: Text(
                'Capacity Reached',
                style: TextStyle(color: Colors.orange, fontSize: 11, fontWeight: FontWeight.bold),
              ),
            ),
        ],
      ),
    );
  }
}

class _GroupHeader extends StatelessWidget {
  const _GroupHeader({required this.type, required this.count});

  final OrderType type;
  final int count;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 10, 4, 6),
      child: Row(
        children: [
          OrderTypeBadge(type: type),
          const SizedBox(width: 8),
          Text(
            '($count)',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                  fontWeight: FontWeight.w600,
                ),
          ),
        ],
      ),
    );
  }
}

class _ActiveOrderCard extends ConsumerWidget {
  const _ActiveOrderCard({required this.order});

  final DriverOrder order;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 4),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => context.push(AppRoutes.orderDetail(order.id), extra: order),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  OrderTypeBadge(type: order.type),
                  const SizedBox(width: 8),
                  Text('#${shortId(order.id)}', style: theme.textTheme.labelMedium),
                  const Spacer(),
                  StatusBadge(status: order.status),
                ],
              ),
              const SizedBox(height: 8),
              _row(Icons.arrow_upward, order.pickupAddress.isEmpty ? 'Pickup' : order.pickupAddress),
              const SizedBox(height: 4),
              _row(Icons.arrow_downward, _destLabel(order)),
              const SizedBox(height: 8),
              Row(
                children: [
                  const Icon(Icons.route_outlined, size: 14, color: Colors.grey),
                  const SizedBox(width: 4),
                  Text('${order.distanceKm.toStringAsFixed(1)} km',
                      style: theme.textTheme.bodySmall),
                  const Spacer(),
                  Icon(
                    order.type == OrderType.ride ? Icons.payments_outlined : Icons.savings_outlined,
                    size: 14,
                    color: Colors.green,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    formatRupiah(order.driverEarning > 0 ? order.driverEarning : order.estimatedFare),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: Colors.green.shade800,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  static String _destLabel(DriverOrder o) {
    if (o.type == OrderType.send && o.hasStops) {
      final done = o.stopsCompleted;
      return 'Stop ${done + 1}/${o.stops.length} • ${o.nextUndeliveredStop?.address.isNotEmpty == true ? o.nextUndeliveredStop!.address : 'Tujuan'}';
    }
    return o.deliveryAddress.isEmpty ? 'Tujuan' : o.deliveryAddress;
  }
}

Widget _row(IconData icon, String text) {
  return Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Icon(icon, size: 14, color: Colors.grey),
      const SizedBox(width: 6),
      Expanded(child: Text(text, style: const TextStyle(fontSize: 13), overflow: TextOverflow.ellipsis)),
    ],
  );
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(top: 10),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: Colors.red.shade50,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(message, style: TextStyle(color: Colors.red.shade800, fontSize: 12)),
    );
  }
}