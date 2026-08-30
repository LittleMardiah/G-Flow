import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../config/constants.dart';
import '../config/router.dart';
import '../models/merchant.dart';
import '../providers/analytics_provider.dart';
import '../providers/auth_provider.dart';
import '../providers/order_provider.dart';
import '../widgets/kpi_card.dart';

class DashboardScreen extends ConsumerStatefulWidget {
  const DashboardScreen({super.key});

  @override
  ConsumerState<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends ConsumerState<DashboardScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(orderProvider.notifier).load();
    });
  }

  Future<void> _toggleOpen(MerchantProfile? profile) async {
    if (profile == null) return;
    final messenger = ScaffoldMessenger.of(context);
    final ok = await ref.read(merchantServiceProvider).updateProfile(
          profile.id,
          isOpen: !profile.isOpen,
        );
    if (!mounted) return;
    ref.invalidate(merchantProfileProvider);
    messenger.showSnackBar(
      SnackBar(content: Text(ok.isOpen ? 'Toko sekarang buka' : 'Toko sekarang tutup')),
    );
  }

  @override
  Widget build(BuildContext context) {
    final profileAsync = ref.watch(merchantProfileProvider);
    final analytics = ref.watch(analyticsTodayProvider);
    final orders = ref.watch(orderProvider);
    final theme = Theme.of(context);

    final profile = profileAsync.value;
    final profileLoading = profileAsync.isLoading;

    final activeOrders = orders.orders.where((o) =>
        o.displayStatus == 'WAITING' ||
        o.displayStatus == 'CONFIRMED' ||
        o.displayStatus == 'PREPARING').length;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Dashboard'),
        actions: [
          IconButton(
            icon: const Icon(Icons.logout),
            tooltip: 'Logout',
            onPressed: () async {
              await ref.read(authProvider.notifier).logout();
              if (!context.mounted) return;
              context.go(AppRoutes.login);
            },
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(merchantProfileProvider);
          await ref.read(orderProvider.notifier).load();
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(
              profile == null ? '${greetingForHour(DateTime.now())}, Merchant' : '${greetingForHour(DateTime.now())}, ${profile.merchantName}',
              style: theme.textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w800),
            ),
            if (profileLoading) const LinearProgressIndicator(),
            if (profile != null && !profile.isActive)
              Card(
                color: theme.colorScheme.errorContainer,
                child: ListTile(
                  leading: const Icon(Icons.verified_outlined),
                  title: const Text('Status: PENDING_VERIFICATION'),
                  subtitle: const Text('Toko belum aktif. Selesaikan verifikasi agar bisa menerima pesanan.'),
                ),
              ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: KpiCard(
                    title: 'Total Orders Hari Ini',
                    value: '${analytics.totalOrders}',
                    icon: Icons.receipt_long_outlined,
                    loading: profileLoading,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: KpiCard(
                    title: 'Revenue Hari Ini',
                    value: formatRupiah(analytics.totalRevenue),
                    icon: Icons.payments_outlined,
                    color: Colors.green,
                    loading: profileLoading,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: KpiCard(
                    title: 'Pesanan Aktif',
                    value: '$activeOrders',
                    icon: Icons.local_fire_department_outlined,
                    color: Colors.orange,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: KpiCard(
                    title: 'Rating',
                    value: profile == null ? '—' : '${profile.avgRating.toStringAsFixed(1)}  (${profile.totalReviews})',
                    icon: Icons.star_outline,
                    color: Colors.amber,
                    loading: profileLoading,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            if (profile != null)
              Card(
                elevation: 0,
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
                child: SwitchListTile(
                  value: profile.isOpen,
                  onChanged: (_) => _toggleOpen(profile),
                  title: const Text('Toko Buka'),
                  subtitle: Text(profile.isOpen ? 'Menerima pesanan' : 'Sedang tutup'),
                  secondary: Icon(
                    profile.isOpen ? Icons.storefront : Icons.storefront_outlined,
                    color: profile.isOpen ? Colors.green : Colors.grey,
                  ),
                ),
              ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: _QuickAction(
                    icon: Icons.list_alt,
                    label: 'View Orders',
                    onTap: () => context.go(AppRoutes.orders),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _QuickAction(
                    icon: Icons.restaurant_menu,
                    label: 'Manage Menu',
                    onTap: () => context.go(AppRoutes.menuManagement),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _QuickAction(
                    icon: Icons.bar_chart,
                    label: 'Analytics',
                    onTap: () => context.go(AppRoutes.analytics),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),
          ],
        ),
      ),
    );
  }
}

class _QuickAction extends StatelessWidget {
  const _QuickAction({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 16),
          child: Column(
            children: [
              Icon(icon, color: theme.colorScheme.primary),
              const SizedBox(height: 8),
              Text(label, style: theme.textTheme.bodySmall?.copyWith(fontWeight: FontWeight.w700)),
            ],
          ),
        ),
      ),
    );
  }
}