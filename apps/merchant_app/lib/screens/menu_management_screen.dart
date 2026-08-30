import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../config/constants.dart';
import '../models/merchant_item.dart';
import '../models/merchant_menu.dart';
import '../providers/menu_provider.dart';

class MenuManagementScreen extends ConsumerStatefulWidget {
  const MenuManagementScreen({super.key});

  @override
  ConsumerState<MenuManagementScreen> createState() => _MenuManagementScreenState();
}

class _MenuManagementScreenState extends ConsumerState<MenuManagementScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(menuProvider.notifier).load();
    });
  }

  Future<void> _addMenu() async {
    final nameController = TextEditingController();
    final descController = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Tambah Menu'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: nameController, decoration: const InputDecoration(labelText: 'Nama menu')),
            TextField(controller: descController, decoration: const InputDecoration(labelText: 'Deskripsi')),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Batal')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Simpan')),
        ],
      ),
    );
    if (ok != true) return;
    final name = nameController.text.trim();
    if (name.isEmpty) return;
    final success = await ref.read(menuProvider.notifier).createMenu(name: name, description: descController.text.trim());
    if (!mounted) return;
    _showResult(success);
  }

  Future<void> _editMenu(MerchantMenu menu) async {
    final nameController = TextEditingController(text: menu.name);
    final descController = TextEditingController(text: menu.description ?? '');
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Edit Menu'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: nameController, decoration: const InputDecoration(labelText: 'Nama menu')),
            TextField(controller: descController, decoration: const InputDecoration(labelText: 'Deskripsi')),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Batal')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Simpan')),
        ],
      ),
    );
    if (ok != true) return;
    final success = await ref.read(menuProvider.notifier).updateMenu(menu.id, name: nameController.text.trim(), description: descController.text.trim());
    if (!mounted) return;
    _showResult(success);
  }

  Future<void> _deleteMenu(MerchantMenu menu) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus Menu?'),
        content: Text('Menu "${menu.name}" beserta seluruh item-nya?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Batal')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Theme.of(context).colorScheme.error),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final success = await ref.read(menuProvider.notifier).deleteMenu(menu.id);
    if (!mounted) return;
    _showResult(success);
  }

  Future<void> _deleteItem(MerchantItem item) async {
    final safe = item.outOfStock || !item.isAvailable;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus Item?'),
        content: Text(safe
            ? 'Item "${item.name}" akan dihapus.'
            : 'Item "${item.name}" masih aktif & stok tersedia. Hapus tetap?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Batal')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Theme.of(context).colorScheme.error),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final success = await ref.read(menuProvider.notifier).deleteItem(item.id);
    if (!mounted) return;
    _showResult(success);
  }

  void _showResult(bool success) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(success ? 'Berhasil diperbarui' : 'Gagal: ${ref.read(menuProvider).error ?? 'unknown'}')),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(menuProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Manage Menu')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _addMenu,
        icon: const Icon(Icons.add),
        label: const Text('Tambah Menu'),
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.read(menuProvider.notifier).load(),
        child: _buildBody(state, theme),
      ),
    );
  }

  Widget _buildBody(MenuState state, ThemeData theme) {
    if (state.isLoading && state.menus.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.menus.isEmpty) {
      return Center(child: Text(state.error!));
    }
    if (state.menus.isEmpty) {
      return const Center(child: Text('Belum ada menu. Tekan + untuk menambah.'));
    }
    return ListView.builder(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(12),
      itemCount: state.menus.length,
      itemBuilder: (context, index) {
        final menu = state.menus[index];
        final items = state.items.where((i) => i.menuId == menu.id).toList();
        return Card(
          elevation: 0,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          child: ExpansionTile(
            shape: const Border(),
            collapsedShape: const Border(),
            title: Row(
              children: [
                Expanded(child: Text(menu.name, style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800))),
                if (!menu.isActive) const StatusPill(text: 'NONAKTIF'),
              ],
            ),
            subtitle: Text('${items.length} item'),
            trailing: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                IconButton(
                  icon: const Icon(Icons.edit_outlined, size: 20),
                  onPressed: () => _editMenu(menu),
                ),
                IconButton(
                  icon: const Icon(Icons.delete_outline, size: 20),
                  onPressed: () => _deleteMenu(menu),
                ),
              ],
            ),
            children: [
              for (final item in items)
                ListTile(
                  leading: item.isAvailable
                      ? const Icon(Icons.fastfood, color: Colors.green)
                      : const Icon(Icons.fastfood, color: Colors.grey),
                  title: Text(item.name),
                  subtitle: Text(
                    '${formatRupiah(item.price)}  •  stok ${item.stock}${!item.isAvailable ? '  •  TIDAK AKTIF' : ''}',
                  ),
                  trailing: IconButton(
                    icon: const Icon(Icons.delete_outline, size: 20),
                    onPressed: () => _deleteItem(item),
                  ),
                  onTap: () => context.go('/menu/${menu.id}/items/${item.id}'),
                ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                child: SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    onPressed: () => context.go('/menu/${menu.id}/items/new'),
                    icon: const Icon(Icons.add),
                    label: const Text('Tambah Item'),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class StatusPill extends StatelessWidget {
  const StatusPill({super.key, required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: const Color(0xFFFFEBEE),
        borderRadius: BorderRadius.circular(10),
      ),
      child: const Text('NONAKTIF',
          style: TextStyle(color: Colors.red, fontSize: 11, fontWeight: FontWeight.w700)),
    );
  }
}