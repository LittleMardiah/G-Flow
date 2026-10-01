import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/order_provider.dart';
import '../widgets/order_card.dart';

class OrdersListScreen extends ConsumerStatefulWidget {
  const OrdersListScreen({super.key});

  @override
  ConsumerState<OrdersListScreen> createState() => _OrdersListScreenState();
}

class _OrdersListScreenState extends ConsumerState<OrdersListScreen> {
  // Tab diurut: proses merchant dulu, terminal terakhir. 'CANCELLED' =
  // order yang di-reject/dibatalkan; backend tidak punya merchant_status
  // 'CANCELLED' (CHECK constraint), jadi tab ini difilter lewat `status`
  // order (fix TD-140).
  static const _filters = ['ALL', 'WAITING', 'CONFIRMED', 'PREPARING', 'READY', 'DELIVERED', 'CANCELLED'];

  static const _labels = {'ALL': 'All', 'CANCELLED': 'Dibatalkan'};

  static String _labelFor(String f) => _labels[f] ?? f;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(orderProvider.notifier).load();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(orderProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Orders')),
      body: Column(
        children: [
          SizedBox(
            height: 56,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              children: _filters.map((f) {
                final selected = state.filter == f || (state.filter == null && f == 'ALL');
                return Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  child: ChoiceChip(
                    label: Text(_labelFor(f)),
                    selected: selected,
                    onSelected: (_) => ref.read(orderProvider.notifier).load(status: f),
                  ),
                );
              }).toList(),
            ),
          ),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () => ref.read(orderProvider.notifier).load(),
              child: _buildBody(state, theme),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBody(OrdersState state, ThemeData theme) {
    if (state.isLoading && state.orders.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.orders.isEmpty) {
      return ListView(
        children: [
          const SizedBox(height: 32),
          Icon(Icons.cloud_off, size: 48, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(height: 8),
          Text(state.error!, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          Center(
            child: FilledButton.icon(
              onPressed: () => ref.read(orderProvider.notifier).load(),
              icon: const Icon(Icons.refresh),
              label: const Text('Coba Lagi'),
            ),
          ),
        ],
      );
    }
    // visibleOrders menyaring order terminal (CANCELLED/DELIVERED/SETTLED)
    // dari tab proses — backend masih mengirim order reject karena
    // merchant_status-nya 'WAITING' (fix TD-140).
    final orders = List.of(state.visibleOrders)
      ..sort((a, b) => (b.createdAt ?? DateTime(0)).compareTo(a.createdAt ?? DateTime(0)));
    if (orders.isEmpty) {
      return Center(
        child: Text(state.filter == null || state.filter == 'ALL' ? 'Belum ada pesanan' : 'Tidak ada pesanan di tab ini'),
      );
    }
    return ListView.builder(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(12),
      itemCount: orders.length,
      itemBuilder: (context, index) {
        final order = orders[index];
        return Column(
          children: [
            OrderCard(order: order, onTap: () => context.go('/orders/${order.id}')),
            const SizedBox(height: 12),
          ],
        );
      },
    );
  }
}