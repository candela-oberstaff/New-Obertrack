import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/biometric_service.dart';
import '../../core/theme.dart';
import '../auth/auth_controller.dart';
import '../chat/chat_repository.dart';
import '../chat/chat_socket.dart';
import '../chat/chats_screen.dart';
import 'home_screen.dart';
import '../notifications/notifications_socket.dart';
import '../profile/profile_screen.dart';
import '../tasks/tasks_screen.dart';
import '../work_hours/work_hours_screen.dart';

/// Cada pestaña visible según los permisos del usuario.
class _Tab {
  const _Tab({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.screen,
    this.showBadge = false,
  });
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final Widget screen;
  final bool showBadge;
}

class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});

  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _index = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // Abre los WebSockets (notificaciones + chats) para toda la sesión.
      ref.read(notificationsSocketProvider).start();
      ref.read(channelsSocketProvider).start();
      // Ofrece activar la huella la primera vez (si el equipo la soporta).
      _maybeOfferBiometrics();
    });
  }

  /// Muestra una sola vez el ofrecimiento de activar el inicio con huella.
  Future<void> _maybeOfferBiometrics() async {
    final prefs = ref.read(biometricPrefsProvider);
    if (await prefs.wasOffered) return;
    if (await prefs.isEnabled) return;
    final available = await ref.read(biometricServiceProvider).isAvailable;
    if (!available || !mounted) return;

    await prefs.markOffered();
    if (!mounted) return;
    final wants = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        icon: const Icon(Icons.fingerprint, color: Brand.blueViolet, size: 40),
        title: const Text('Inicio con huella'),
        content: const Text(
          '¿Quieres entrar más rápido usando tu huella la próxima vez, '
          'sin escribir tu contraseña?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Ahora no'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Activar'),
          ),
        ],
      ),
    );
    if (wants != true || !mounted) return;

    final ok = await ref.read(authControllerProvider.notifier).enableBiometrics();
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(ok
          ? 'Listo, la próxima vez podrás entrar con tu huella'
          : 'No se pudo activar la huella'),
    ));
  }

  List<_Tab> _tabsFor(WidgetRef ref) {
    final user = ref.read(currentUserProvider);
    final tabs = <_Tab>[
      const _Tab(
        icon: Icons.home_outlined,
        selectedIcon: Icons.home,
        label: 'Inicio',
        screen: HomeScreen(),
      ),
    ];
    // Módulos según permisos (ausencia de permisos = acceso histórico completo).
    if (user?.canView('tasks') ?? true) {
      tabs.add(const _Tab(
        icon: Icons.task_alt_outlined,
        selectedIcon: Icons.task_alt,
        label: 'Tareas',
        screen: TasksScreen(),
      ));
    }
    if (user?.canView('hours') ?? true) {
      tabs.add(const _Tab(
        icon: Icons.schedule_outlined,
        selectedIcon: Icons.schedule,
        label: 'Horas',
        screen: WorkHoursScreen(),
      ));
    }
    if (user?.canView('chat') ?? true) {
      tabs.add(const _Tab(
        icon: Icons.forum_outlined,
        selectedIcon: Icons.forum,
        label: 'Chats',
        screen: ChatsScreen(),
        showBadge: true,
      ));
    }
    tabs.add(const _Tab(
      icon: Icons.person_outline,
      selectedIcon: Icons.person,
      label: 'Perfil',
      screen: ProfileScreen(),
    ));
    return tabs;
  }

  @override
  Widget build(BuildContext context) {
    final tabs = _tabsFor(ref);
    final safeIndex = _index.clamp(0, tabs.length - 1);
    // El badge de la pestaña Chats usa el total de mensajes sin leer.
    final unread = ref.watch(channelsUnreadProvider).maybeWhen(
          data: (c) => c,
          orElse: () => 0,
        );

    return Scaffold(
      body: IndexedStack(
        index: safeIndex,
        children: [for (final t in tabs) t.screen],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: safeIndex,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: [
          for (final t in tabs)
            NavigationDestination(
              icon: t.showBadge && unread > 0
                  ? Badge(
                      label: Text('$unread'),
                      child: Icon(t.icon),
                    )
                  : Icon(t.icon),
              selectedIcon: t.showBadge && unread > 0
                  ? Badge(
                      label: Text('$unread'),
                      child: Icon(t.selectedIcon),
                    )
                  : Icon(t.selectedIcon),
              label: t.label,
            ),
        ],
      ),
    );
  }
}
