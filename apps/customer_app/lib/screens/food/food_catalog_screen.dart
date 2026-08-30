import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:latlong2/latlong.dart';

import '../../models/merchant.dart';
import '../../providers/merchant_provider.dart';

/// Food Catalog Screen (ROADMAP 3.8 A & HALAMAN.txt 4.1).
///
/// Daftar merchant terdekat dengan pencarian, sortir, dan toggle peta/list.
class FoodCatalogScreen extends ConsumerStatefulWidget {
  const FoodCatalogScreen({super.key});

  @override
  ConsumerState<FoodCatalogScreen> createState() => _FoodCatalogScreenState();
}

class _FoodCatalogScreenState extends ConsumerState<FoodCatalogScreen> {
  bool _mapView = false;
  final MapController _mapController = MapController();

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(merchantListProvider.notifier).load());
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(merchantListProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Food Delivery'),
        actions: [
          IconButton(
            tooltip: _mapView ? 'Tampilkan daftar' : 'Tampilkan peta',
            icon: Icon(_mapView ? Icons.list : Icons.map_outlined),
            onPressed: () => setState(() => _mapView = !_mapView),
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
            child: TextField(
              decoration: const InputDecoration(
                labelText: 'Cari merchant / kategori…',
                prefixIcon: Icon(Icons.search),
                border: OutlineInputBorder(),
                isDense: true,
              ),
              onChanged: (value) =>
                  ref.read(merchantListProvider.notifier).setQuery(value),
            ),
          ),
          _buildSortBar(theme),
          Expanded(child: _buildBody(state)),
        ],
      ),
    );
  }

  Widget _buildSortBar(ThemeData theme) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          const Text('Urutkan: ', style: TextStyle(fontSize: 12)),
          ActionChip(
            label: const Text('Terdekat'),
            onPressed: () => ref.read(merchantListProvider.notifier).sortByDistance(),
          ),
          const SizedBox(width: 6),
          ActionChip(
            label: const Text('Rating'),
            onPressed: () => ref.read(merchantListProvider.notifier).sortByRating(),
          ),
        ],
      ),
    );
  }

  Widget _buildBody(MerchantListState state) {
    if (state.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null) {
      return _ErrorView(
        message: state.error!,
        onRetry: () => ref.read(merchantListProvider.notifier).load(),
      );
    }
    final merchants = state.visible;
    if (merchants.isEmpty) {
      return const Center(child: Text('Tidak ada merchant ditemukan.'));
    }
    if (_mapView) {
      return _buildMap(merchants);
    }
    return _buildList(merchants);
  }

  Widget _buildList(List<Merchant> merchants) {
    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: merchants.length,
      separatorBuilder: (_, _) => const SizedBox(height: 10),
      itemBuilder: (context, i) {
        final m = merchants[i];
        return _MerchantCard(
          merchant: m,
          onTap: () => _openMerchant(m),
        );
      },
    );
  }

  Widget _buildMap(List<Merchant> merchants) {
    final center = LatLng(
      merchants.isNotEmpty ? merchants.first.latitude : -6.2088,
      merchants.isNotEmpty ? merchants.first.longitude : 106.8456,
    );
    return FlutterMap(
      mapController: _mapController,
      options: MapOptions(
        initialCenter: center,
        initialZoom: 12,
        interactionOptions: const InteractionOptions(
          flags: InteractiveFlag.all & ~InteractiveFlag.rotate,
        ),
      ),
      children: [
        TileLayer(
          urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
          userAgentPackageName: 'com.gflow.customer_app',
        ),
        MarkerLayer(
          markers: merchants
              .map((m) => Marker(
                    point: LatLng(m.latitude, m.longitude),
                    width: 44,
                    height: 44,
                    child: GestureDetector(
                      onTap: () => _openMerchant(m),
                      child: const Tooltip(
                        message: 'Tap untuk detail',
                        child: Icon(Icons.location_pin, color: Colors.red, size: 44),
                      ),
                    ),
                  ))
              .toList(),
        ),
      ],
    );
  }

  void _openMerchant(Merchant m) {
    Navigator.of(context).pushNamed('/food-merchant-detail', arguments: m);
  }
}

class _MerchantCard extends StatelessWidget {
  const _MerchantCard({required this.merchant, required this.onTap});

  final Merchant merchant;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              CircleAvatar(
                radius: 26,
                backgroundColor: theme.colorScheme.primaryContainer,
                child: Icon(Icons.storefront, color: theme.colorScheme.onPrimaryContainer),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            merchant.name,
                            style: theme.textTheme.titleMedium,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        if (!merchant.isOpen)
                          const Chip(
                            label: Text('Tutup', style: TextStyle(fontSize: 10)),
                            visualDensity: VisualDensity.compact,
                            backgroundColor: Colors.red,
                            labelStyle: TextStyle(color: Colors.white),
                          ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        const Icon(Icons.star, color: Colors.amber, size: 16),
                        const SizedBox(width: 2),
                        Text(
                          merchant.rating.toStringAsFixed(1),
                          style: theme.textTheme.bodySmall,
                        ),
                        if (merchant.distanceKm > 0) ...[
                          const SizedBox(width: 8),
                          Text(
                            '${merchant.distanceKm.toStringAsFixed(1)} km',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ],
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            merchant.averageDeliveryTime > 0
                                ? '~${merchant.averageDeliveryTime} menit'
                                : merchant.category,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right),
            ],
          ),
        ),
      ),
    );
  }
}

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
            const Text('Gagal memuat merchant', style: TextStyle(fontSize: 16)),
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
